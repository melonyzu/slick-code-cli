package ollama

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/melonyzu/slick-code-cli/internal/auth"
	"github.com/melonyzu/slick-code-cli/internal/provider"
	"github.com/melonyzu/slick-code-cli/pkg/types"
)

func activated(t *testing.T, handler http.Handler) *Provider {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	t.Setenv(endpointEnv, server.URL)
	t.Setenv(ollamaHostEnv, "")
	p, err := New(server.Client(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Activate(context.Background(), auth.Credential{}); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestEndpointDiscoveryAndConfiguration(t *testing.T) {
	t.Setenv(endpointEnv, "")
	t.Setenv(ollamaHostEnv, "localhost:11435")
	p, err := New(http.DefaultClient, nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.Endpoint() != "http://localhost:11435" {
		t.Fatalf("endpoint = %q", p.Endpoint())
	}
	t.Setenv(endpointEnv, "https://example.test/ollama/")
	p, err = New(http.DefaultClient, nil)
	if err != nil || p.Endpoint() != "https://example.test/ollama" {
		t.Fatalf("endpoint = %q error = %v", p.Endpoint(), err)
	}
	t.Setenv(endpointEnv, "https://user:secret@example.test")
	if _, err := New(http.DefaultClient, nil); types.KindOf(err) != types.ErrorKindInvalidConfig {
		t.Fatalf("invalid endpoint error = %v", err)
	}
}

func TestRegistrationAndNoAuthentication(t *testing.T) {
	p, err := New(http.DefaultClient, nil)
	if err != nil {
		t.Fatal(err)
	}
	registry := provider.NewRegistry()
	if err := registry.Register(p); err != nil {
		t.Fatal(err)
	}
	if methods := p.AuthMethods(); len(methods) != 1 || methods[0] != auth.MethodNone {
		t.Fatalf("auth methods = %v", methods)
	}
	flow, err := p.NewFlow(auth.MethodNone)
	if err != nil || flow.Method() != auth.MethodNone {
		t.Fatalf("flow = %v error = %v", flow, err)
	}
	if _, err := p.NewFlow(auth.MethodAPIKey); types.KindOf(err) != types.ErrorKindUnsupportedCapability {
		t.Fatalf("API-key flow error = %v", err)
	}
	if err := p.Activate(context.Background(), auth.Credential{Secret: "forbidden"}); types.KindOf(err) != types.ErrorKindAuthentication {
		t.Fatalf("credential error = %v", err)
	}
}

func TestHealthModelsCapabilitiesAndValidation(t *testing.T) {
	var authorization string
	p := activated(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization = r.Header.Get("Authorization")
		switch r.URL.Path {
		case "/api/tags":
			fmt.Fprint(w, `{"models":[
				{"name":"qwen3-coder:30b"},{"model":"deepseek-r1:32b"},{"name":"nomic-embed-text:latest"}
			]}`)
		case "/api/show":
			var request apiShowRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Error(err)
			}
			switch request.Model {
			case "qwen3-coder:30b":
				fmt.Fprint(w, `{"capabilities":["completion","tools"]}`)
			case "deepseek-r1:32b":
				fmt.Fprint(w, `{"capabilities":["completion","thinking"]}`)
			default:
				fmt.Fprint(w, `{"capabilities":["embedding"]}`)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	models, err := p.Models(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if authorization != "" {
		t.Fatal("Ollama request unexpectedly included authorization")
	}
	if len(models) != 2 || models[0].ID != "deepseek-r1:32b" || models[1].ID != "qwen3-coder:30b" {
		t.Fatalf("models = %+v", models)
	}
	if !models[0].Capabilities.Has(types.CapabilityReasoning) ||
		!models[1].Capabilities.Has(types.CapabilityTools) {
		t.Fatalf("capabilities = %+v", models)
	}
	if err := p.ValidateModel(context.Background(), "qwen3-coder:30b"); err != nil {
		t.Fatal(err)
	}
	if err := p.ValidateModel(context.Background(), "missing:latest"); types.KindOf(err) != types.ErrorKindValidation || !strings.Contains(err.Error(), "ollama pull missing:latest") {
		t.Fatalf("missing model error = %v", err)
	}
}

func TestCompleteUsesExactModelAndToolCalls(t *testing.T) {
	var captured apiChatRequest
	p := activated(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			fmt.Fprint(w, `{"models":[{"name":"registry.example/team/qwen:30b"}]}`)
		case "/api/show":
			fmt.Fprint(w, `{"capabilities":["completion","tools","thinking"]}`)
		case "/api/chat":
			if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
				t.Error(err)
			}
			fmt.Fprint(w, `{
				"model":"registry.example/team/qwen:30b","done":true,"done_reason":"stop",
				"message":{"role":"assistant","thinking":"checking","tool_calls":[{
					"function":{"name":"git_status","arguments":{}}
				}]},"prompt_eval_count":7,"eval_count":3
			}`)
		}
	}))
	if err := p.ValidateModel(context.Background(), "registry.example/team/qwen:30b"); err != nil {
		t.Fatal(err)
	}
	response, err := p.Complete(context.Background(), types.Request{
		Model:    "registry.example/team/qwen:30b",
		Messages: []types.Message{types.NewTextMessage(types.RoleUser, "status")},
		Tools:    []types.Tool{{Name: "git_status", InputSchema: json.RawMessage(`{"type":"object"}`)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if captured.Model != "registry.example/team/qwen:30b" || len(captured.Tools) != 1 {
		t.Fatalf("request = %+v", captured)
	}
	if response.StopReason != types.StopReasonToolUse || response.Usage.TotalTokens() != 10 {
		t.Fatalf("response = %+v", response)
	}
	call := response.Message.Parts[1].(types.ToolCallPart).Call
	if call.ID != "ollama_call_1" || call.Name != "git_status" || string(call.Input) != "{}" {
		t.Fatalf("call = %+v", call)
	}
}

func TestRetryAndErrorMapping(t *testing.T) {
	var attempts atomic.Int32
	p := activated(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if attempts.Add(1) < 3 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, `{"error":"loading"}`)
			return
		}
		fmt.Fprint(w, `{"models":[]}`)
	}))
	if attempts.Load() != 3 {
		t.Fatalf("activation attempts = %d", attempts.Load())
	}
	if err := p.ValidateModel(context.Background(), "missing:latest"); types.KindOf(err) != types.ErrorKindValidation {
		t.Fatalf("validation error = %v", err)
	}
}

package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
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
	p, err := New(server.Client(), nil)
	if err != nil {
		t.Fatal(err)
	}
	p.baseURL = server.URL
	if err := p.Activate(context.Background(), auth.Credential{
		Provider: types.ProviderOpenAI, Method: auth.MethodAPIKey, Secret: "sk-local-test",
	}); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestProviderRegistrationAndAuthentication(t *testing.T) {
	p, err := New(http.DefaultClient, nil)
	if err != nil {
		t.Fatal(err)
	}
	registry := provider.NewRegistry()
	if err := registry.Register(p); err != nil {
		t.Fatal(err)
	}
	if got, err := registry.Get(types.ProviderOpenAI); err != nil || got != p {
		t.Fatalf("registered provider = %v error = %v", got, err)
	}
	if methods := p.AuthMethods(); len(methods) != 1 || methods[0] != auth.MethodAPIKey {
		t.Fatalf("auth methods = %v", methods)
	}
	flow, err := p.NewFlow(auth.MethodAPIKey)
	if err != nil {
		t.Fatal(err)
	}
	credential, err := flow.(auth.APIKeyFlow).Exchange(context.Background(), " sk-test ")
	if err != nil || credential.Secret.Reveal() != "sk-test" || credential.Provider != types.ProviderOpenAI {
		t.Fatalf("credential = %+v error = %v", credential, err)
	}
	if _, err := flow.(auth.APIKeyFlow).Exchange(context.Background(), "  "); types.KindOf(err) != types.ErrorKindValidation {
		t.Fatalf("empty key error = %v", err)
	}
	if _, err := p.NewFlow(auth.MethodBrowserOAuth); types.KindOf(err) != types.ErrorKindUnsupportedCapability {
		t.Fatalf("unsupported auth error = %v", err)
	}
	if err := p.Activate(context.Background(), auth.Credential{Provider: types.ProviderAnthropic, Secret: "wrong"}); types.KindOf(err) != types.ErrorKindAuthentication {
		t.Fatalf("invalid session error = %v", err)
	}
}

func TestCompleteAndToolCalling(t *testing.T) {
	var captured apiRequest
	p := activated(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer sk-local-test" {
			t.Errorf("unexpected authenticated request: %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Error(err)
		}
		fmt.Fprint(w, `{
			"id":"chatcmpl-local","model":"gpt-5.4",
			"choices":[{"index":0,"finish_reason":"tool_calls","message":{"role":"assistant","tool_calls":[{
				"id":"call_1","type":"function","function":{"name":"git_status","arguments":"{}"}
			}]}}],
			"usage":{"prompt_tokens":10,"completion_tokens":4,"total_tokens":14}
		}`)
	}))

	response, err := p.Complete(context.Background(), types.Request{
		Model: "gpt-5.4",
		Messages: []types.Message{
			types.NewTextMessage(types.RoleSystem, "Use tools safely."),
			types.NewTextMessage(types.RoleUser, "Check status"),
		},
		Tools: []types.Tool{{Name: "git_status", Description: "Status", InputSchema: json.RawMessage(`{"type":"object"}`)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(captured.Tools) != 1 || captured.Tools[0].Function.Name != "git_status" {
		t.Fatalf("captured tools = %+v", captured.Tools)
	}
	if response.StopReason != types.StopReasonToolUse || response.Usage.TotalTokens() != 14 {
		t.Fatalf("response = %+v", response)
	}
	call, ok := response.Message.Parts[0].(types.ToolCallPart)
	if !ok || call.Call.ID != "call_1" || call.Call.Name != "git_status" || string(call.Call.Input) != "{}" {
		t.Fatalf("tool call = %+v", response.Message.Parts)
	}
}

func TestConfigurationAndSessionValidation(t *testing.T) {
	t.Setenv(baseURLEnv, "https://user:secret@example.com?bad=true")
	if _, err := New(http.DefaultClient, nil); types.KindOf(err) != types.ErrorKindInvalidConfig {
		t.Fatalf("invalid base URL error = %v", err)
	}
	t.Setenv(baseURLEnv, "")
	if _, err := New(nil, nil); types.KindOf(err) != types.ErrorKindInvalidConfig {
		t.Fatalf("nil HTTP client error = %v", err)
	}
	p, err := New(http.DefaultClient, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Models(context.Background()); types.KindOf(err) != types.ErrorKindAuthentication {
		t.Fatalf("inactive provider error = %v", err)
	}
	if err := p.Activate(context.Background(), auth.Credential{
		Provider: types.ProviderOpenAI, Method: auth.MethodAPIKey, Secret: "temporary",
	}); err != nil {
		t.Fatal(err)
	}
	if err := p.Deactivate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Models(context.Background()); types.KindOf(err) != types.ErrorKindAuthentication {
		t.Fatalf("deactivated provider error = %v", err)
	}
}

func TestErrorKinds(t *testing.T) {
	cases := []struct {
		status int
		kind   types.ErrorKind
	}{
		{http.StatusBadRequest, types.ErrorKindValidation},
		{http.StatusUnauthorized, types.ErrorKindAuthentication},
		{http.StatusGatewayTimeout, types.ErrorKindTimeout},
		{http.StatusInternalServerError, types.ErrorKindProvider},
	}
	for _, testCase := range cases {
		p := activated(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(testCase.status)
			fmt.Fprint(w, `{"error":{"message":"local failure","type":"test_error"}}`)
		}))
		_, err := p.Models(context.Background())
		if types.KindOf(err) != testCase.kind {
			t.Fatalf("status %d: error=%v kind=%s", testCase.status, err, types.KindOf(err))
		}
	}
}

func TestStructuredLogsNeverContainAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"data":[]}`)
	}))
	defer server.Close()
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	p, err := New(server.Client(), logger)
	if err != nil {
		t.Fatal(err)
	}
	p.baseURL = server.URL
	const secret = "sk-sensitive-local-test"
	if err := p.Activate(context.Background(), auth.Credential{
		Provider: types.ProviderOpenAI, Method: auth.MethodAPIKey, Secret: auth.Secret(secret),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Models(context.Background()); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(logs.String(), secret) || strings.Contains(strings.ToLower(logs.String()), "authorization") {
		t.Fatalf("structured logs exposed credential material: %s", logs.String())
	}
}

func TestModelsAndCapabilities(t *testing.T) {
	p := activated(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"data":[
			{"id":"text-embedding-3-small"},{"id":"gpt-4o-mini-transcribe"},
			{"id":"gpt-4o"},{"id":"gpt-5.4"},{"id":"o1-preview"}
		]}`)
	}))
	models, err := p.Models(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 3 {
		t.Fatalf("models = %+v", models)
	}
	byID := make(map[string]types.Model, len(models))
	for _, model := range models {
		byID[model.ID] = model
	}
	if !byID["gpt-4o"].Capabilities.Has(types.CapabilityTools) ||
		!byID["gpt-5.4"].Capabilities.Has(types.CapabilityReasoning) ||
		byID["o1-preview"].Capabilities.Has(types.CapabilityTools) {
		t.Fatalf("capabilities = %+v", byID)
	}
}

func TestRetriesAndErrorMapping(t *testing.T) {
	var calls atomic.Int32
	p := activated(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) < 3 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"error":{"message":"slow down","type":"rate_limit_error"}}`)
			return
		}
		fmt.Fprint(w, `{"model":"gpt-4o","choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{}}`)
	}))
	response, err := p.Complete(context.Background(), types.Request{Model: "gpt-4o"})
	if err != nil || response.Message.Text() != "ok" || calls.Load() != 3 {
		t.Fatalf("response=%+v calls=%d error=%v", response, calls.Load(), err)
	}

	unauthorized := activated(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":{"message":"bad key","type":"invalid_request_error"}}`)
	}))
	if _, err := unauthorized.Models(context.Background()); types.KindOf(err) != types.ErrorKindAuthentication {
		t.Fatalf("authentication error = %v", err)
	}
}

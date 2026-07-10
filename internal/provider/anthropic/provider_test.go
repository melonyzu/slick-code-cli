package anthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/melonyzu/slick-code-cli/internal/auth"
	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// activated returns a Provider activated against a test server.
func activated(t *testing.T, handler http.Handler) *Provider {
	t.Helper()

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	p := New(srv.Client(), nil)
	p.baseURL = srv.URL
	if err := p.Activate(context.Background(), auth.Credential{
		Provider: types.ProviderAnthropic,
		Method:   auth.MethodAPIKey,
		Secret:   "sk-test",
	}); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCompleteTranslatesRequestAndResponse(t *testing.T) {
	var captured map[string]any

	p := activated(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" || r.Header.Get("x-api-key") != "sk-test" ||
			r.Header.Get("anthropic-version") != apiVersion {
			t.Errorf("bad request: %s %s %v", r.Method, r.URL, r.Header)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Error(err)
		}
		fmt.Fprint(w, `{
			"model": "claude-sonnet-5",
			"content": [{"type": "text", "text": "Hello!"}],
			"stop_reason": "end_turn",
			"usage": {"input_tokens": 12, "output_tokens": 5}
		}`)
	}))

	resp, err := p.Complete(context.Background(), types.Request{
		Model: "claude-sonnet-5",
		Messages: []types.Message{
			types.NewTextMessage(types.RoleSystem, "Be brief."),
			types.NewTextMessage(types.RoleUser, "Hi"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if captured["system"] != "Be brief." {
		t.Errorf("system prompt not extracted: %v", captured["system"])
	}
	if captured["max_tokens"] != float64(defaultMaxTokens) {
		t.Errorf("max_tokens default not applied: %v", captured["max_tokens"])
	}
	msgs := captured["messages"].([]any)
	if len(msgs) != 1 {
		t.Fatalf("system message leaked into messages: %v", msgs)
	}

	if resp.Message.Text() != "Hello!" || resp.StopReason != types.StopReasonEndTurn {
		t.Errorf("response not translated: %+v", resp)
	}
	if resp.Usage.TotalTokens() != 17 {
		t.Errorf("usage not translated: %+v", resp.Usage)
	}
}

func TestCompleteRejectsUnsupportedContent(t *testing.T) {
	p := activated(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("request must not reach the API")
	}))

	_, err := p.Complete(context.Background(), types.Request{
		Model: "claude-sonnet-5",
		Messages: []types.Message{{
			Role:  types.RoleUser,
			Parts: []types.Part{types.ImagePart{}},
		}},
	})
	if types.KindOf(err) != types.ErrorKindUnsupportedCapability {
		t.Fatalf("want unsupported capability, got %v", err)
	}
}

func TestStreamTranslatesEvents(t *testing.T) {
	p := activated(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req["stream"] != true {
			t.Error("stream flag not set")
		}

		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "event: message_start\n")
		fmt.Fprint(w, `data: {"type":"message_start","message":{"model":"claude-sonnet-5","usage":{"input_tokens":9}}}`+"\n\n")
		fmt.Fprint(w, `data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"Hel"}}`+"\n\n")
		fmt.Fprint(w, `data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"lo"}}`+"\n\n")
		fmt.Fprint(w, `data: {"type":"message_delta","delta":{"stop_reason":"max_tokens"},"usage":{"output_tokens":4}}`+"\n\n")
		fmt.Fprint(w, `data: {"type":"message_stop"}`+"\n\n")
	}))

	seq, err := p.Stream(context.Background(), types.Request{
		Model:    "claude-sonnet-5",
		Messages: []types.Message{types.NewTextMessage(types.RoleUser, "Hi")},
	})
	if err != nil {
		t.Fatal(err)
	}

	var text string
	var done *types.DoneEvent
	for ev, err := range seq {
		if err != nil {
			t.Fatal(err)
		}
		switch e := ev.(type) {
		case types.TextEvent:
			text += e.Text
		case types.DoneEvent:
			done = &e
		}
	}

	if text != "Hello" {
		t.Errorf("text deltas = %q", text)
	}
	if done == nil {
		t.Fatal("no DoneEvent")
	}
	if done.Response.Message.Text() != "Hello" ||
		done.Response.StopReason != types.StopReasonMaxTokens ||
		done.Response.Usage.InputTokens != 9 || done.Response.Usage.OutputTokens != 4 {
		t.Errorf("assembled response wrong: %+v", done.Response)
	}
}

func TestStreamSurfacesAPIError(t *testing.T) {
	p := activated(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, `data: {"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`+"\n\n")
	}))

	seq, err := p.Stream(context.Background(), types.Request{Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	for _, err := range seq {
		if err != nil {
			if types.KindOf(err) != types.ErrorKindProvider {
				t.Fatalf("want provider error, got %v", err)
			}
			return
		}
	}
	t.Fatal("stream yielded no error")
}

func TestErrorTranslation(t *testing.T) {
	cases := []struct {
		status int
		body   string
		want   types.ErrorKind
	}{
		{401, `{"type":"error","error":{"type":"authentication_error","message":"invalid x-api-key"}}`, types.ErrorKindAuthentication},
		{400, `{"type":"error","error":{"type":"invalid_request_error","message":"bad"}}`, types.ErrorKindValidation},
	}

	for _, tc := range cases {
		p := activated(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.status)
			fmt.Fprint(w, tc.body)
		}))

		_, err := p.Complete(context.Background(), types.Request{Model: "m"})
		if types.KindOf(err) != tc.want {
			t.Errorf("status %d: want %s, got %v", tc.status, tc.want, err)
		}
	}
}

func TestRetriesRateLimitThenSucceeds(t *testing.T) {
	var calls atomic.Int32

	p := activated(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"type":"error","error":{"type":"rate_limit_error","message":"slow down"}}`)
			return
		}
		fmt.Fprint(w, `{"model":"m","content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn","usage":{}}`)
	}))

	resp, err := p.Complete(context.Background(), types.Request{Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Message.Text() != "ok" || calls.Load() != 2 {
		t.Fatalf("retry did not recover: calls=%d resp=%+v", calls.Load(), resp)
	}
}

func TestRateLimitExhaustedIsTyped(t *testing.T) {
	p := activated(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"type":"error","error":{"type":"rate_limit_error","message":"slow down"}}`)
	}))

	_, err := p.Complete(context.Background(), types.Request{Model: "m"})
	if types.KindOf(err) != types.ErrorKindRateLimit {
		t.Fatalf("want rate limit error, got %v", err)
	}
}

func TestModelsPaginates(t *testing.T) {
	p := activated(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("after_id") == "" {
			fmt.Fprint(w, `{"data":[{"id":"claude-sonnet-5","display_name":"Claude Sonnet 5"}],"has_more":true}`)
			return
		}
		fmt.Fprint(w, `{"data":[{"id":"claude-haiku-4-5","display_name":"Claude Haiku 4.5"}],"has_more":false}`)
	}))

	models, err := p.Models(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 || models[0].ID != "claude-sonnet-5" || models[1].ID != "claude-haiku-4-5" {
		t.Fatalf("models = %+v", models)
	}
	if !models[0].Capabilities.Has(types.CapabilityStreaming) {
		t.Error("models must advertise streaming")
	}
}

func TestLifecycleAndAuthContracts(t *testing.T) {
	p := New(http.DefaultClient, nil)

	if _, err := p.Complete(context.Background(), types.Request{}); err == nil {
		t.Fatal("inactive provider must refuse requests")
	}

	if got := p.AuthMethods(); len(got) != 1 || got[0] != auth.MethodAPIKey {
		t.Fatalf("auth methods = %v", got)
	}

	flow, err := p.NewFlow(auth.MethodAPIKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := flow.(auth.APIKeyFlow).Exchange(context.Background(), "  "); types.KindOf(err) != types.ErrorKindValidation {
		t.Fatalf("empty key must be rejected: %v", err)
	}
	cred, err := flow.(auth.APIKeyFlow).Exchange(context.Background(), " sk-key ")
	if err != nil || cred.Secret.Reveal() != "sk-key" {
		t.Fatalf("exchange: %v %v", cred, err)
	}

	if _, err := p.NewFlow(auth.MethodBrowserOAuth); types.KindOf(err) != types.ErrorKindUnsupportedCapability {
		t.Fatalf("unsupported method must be typed: %v", err)
	}

	if err := p.Activate(context.Background(), auth.Credential{}); types.KindOf(err) != types.ErrorKindAuthentication {
		t.Fatalf("empty credential must be rejected: %v", err)
	}
}

func TestCheckHealth(t *testing.T) {
	healthy := activated(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":[{"id":"m","display_name":"M"}],"has_more":false}`)
	}))
	if err := healthy.CheckHealth(context.Background()); err != nil {
		t.Fatal(err)
	}

	unhealthy := activated(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"type":"error","error":{"type":"authentication_error","message":"bad key"}}`)
	}))
	if err := unhealthy.CheckHealth(context.Background()); types.KindOf(err) != types.ErrorKindAuthentication {
		t.Fatalf("want authentication error, got %v", err)
	}
}

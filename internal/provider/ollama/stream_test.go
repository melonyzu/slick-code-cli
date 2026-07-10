package ollama

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/melonyzu/slick-code-cli/pkg/types"
)

func TestStreamTextReasoningToolsAndUsage(t *testing.T) {
	p := activated(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			fmt.Fprint(w, `{"models":[]}`)
			return
		}
		fmt.Fprintln(w, `{"model":"deepseek-r1:32b","message":{"role":"assistant","thinking":"plan "},"done":false}`)
		fmt.Fprintln(w, `{"model":"deepseek-r1:32b","message":{"role":"assistant","content":"answer"},"done":false}`)
		fmt.Fprintln(w, `{"model":"deepseek-r1:32b","message":{"role":"assistant","tool_calls":[{"function":{"name":"git_status","arguments":{}}}]},"done":true,"done_reason":"stop","prompt_eval_count":9,"eval_count":4}`)
	}))
	sequence, err := p.Stream(context.Background(), types.Request{Model: "deepseek-r1:32b"})
	if err != nil {
		t.Fatal(err)
	}
	var text, reasoning string
	var call *types.ToolCall
	var done *types.Response
	for event, eventErr := range sequence {
		if eventErr != nil {
			t.Fatal(eventErr)
		}
		switch value := event.(type) {
		case types.TextEvent:
			text += value.Text
		case types.ReasoningEvent:
			reasoning += value.Text
		case types.ToolCallEvent:
			call = &value.Call
		case types.DoneEvent:
			done = &value.Response
		}
	}
	if text != "answer" || reasoning != "plan " || call == nil || call.Name != "git_status" {
		t.Fatalf("text=%q reasoning=%q call=%+v", text, reasoning, call)
	}
	if done == nil || done.StopReason != types.StopReasonToolUse || done.Usage.TotalTokens() != 13 {
		t.Fatalf("done = %+v", done)
	}
}

func TestStreamRejectsMalformedChunk(t *testing.T) {
	p := activated(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			fmt.Fprint(w, `{"models":[]}`)
			return
		}
		fmt.Fprintln(w, `{not-json}`)
	}))
	sequence, err := p.Stream(context.Background(), types.Request{Model: "llama3.1:8b"})
	if err != nil {
		t.Fatal(err)
	}
	for _, eventErr := range sequence {
		if eventErr != nil {
			if types.KindOf(eventErr) != types.ErrorKindProvider {
				t.Fatalf("error = %v", eventErr)
			}
			return
		}
	}
	t.Fatal("stream returned no error")
}

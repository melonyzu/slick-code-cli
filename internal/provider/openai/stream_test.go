package openai

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/melonyzu/slick-code-cli/pkg/types"
)

func TestStreamTextToolCallsAndUsage(t *testing.T) {
	p := activated(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintln(w, `data: {"id":"chatcmpl-stream","model":"gpt-5.4","choices":[{"index":0,"delta":{"role":"assistant","content":"Check "}}]}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, `data: {"choices":[{"index":0,"delta":{"content":"done.","tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"git_status","arguments":"{"}}]}}]}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, `data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"}"}}]},"finish_reason":"tool_calls"}]}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, `data: {"choices":[],"usage":{"prompt_tokens":8,"completion_tokens":5,"total_tokens":13}}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, "data: [DONE]")
		fmt.Fprintln(w)
	}))

	sequence, err := p.Stream(context.Background(), types.Request{Model: "gpt-5.4"})
	if err != nil {
		t.Fatal(err)
	}
	var text string
	var toolCall *types.ToolCall
	var done *types.Response
	for event, eventErr := range sequence {
		if eventErr != nil {
			t.Fatal(eventErr)
		}
		switch value := event.(type) {
		case types.TextEvent:
			text += value.Text
		case types.ToolCallEvent:
			toolCall = &value.Call
		case types.DoneEvent:
			done = &value.Response
		}
	}
	if text != "Check done." || toolCall == nil || toolCall.Name != "git_status" || string(toolCall.Input) != "{}" {
		t.Fatalf("text=%q tool=%+v", text, toolCall)
	}
	if done == nil || done.StopReason != types.StopReasonToolUse || done.Usage.TotalTokens() != 13 ||
		done.Metadata["openai_response_id"] != "chatcmpl-stream" {
		t.Fatalf("done = %+v", done)
	}
}

func TestStreamRejectsMalformedEvent(t *testing.T) {
	p := activated(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintln(w, "data: {not-json}")
	}))
	sequence, err := p.Stream(context.Background(), types.Request{Model: "gpt-4o"})
	if err != nil {
		t.Fatal(err)
	}
	for _, eventErr := range sequence {
		if eventErr != nil {
			if types.KindOf(eventErr) != types.ErrorKindProvider {
				t.Fatalf("stream error = %v", eventErr)
			}
			return
		}
	}
	t.Fatal("stream returned no error")
}

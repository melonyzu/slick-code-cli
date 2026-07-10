package openai

import (
	"encoding/json"
	"testing"

	"github.com/melonyzu/slick-code-cli/pkg/types"
)

func TestRequestTranslatesToolHistory(t *testing.T) {
	request, err := toAPIRequest(types.Request{
		Model: "gpt-5.4", MaxTokens: 2048,
		Messages: []types.Message{
			{Role: types.RoleAssistant, Parts: []types.Part{
				types.TextPart{Text: "Checking."},
				types.ToolCallPart{Call: types.ToolCall{ID: "call_1", Name: "git_status", Input: json.RawMessage(`{}`)}},
			}},
			{Role: types.RoleUser, Parts: []types.Part{
				types.ToolResultPart{Result: types.ToolResult{ToolCallID: "call_1", Content: "clean"}},
			}},
		},
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(request.Messages) != 2 || request.Messages[0].ToolCalls[0].Function.Name != "git_status" ||
		request.Messages[1].Role != "tool" || request.Messages[1].ToolCallID != "call_1" {
		t.Fatalf("messages = %+v", request.Messages)
	}
	if request.MaxCompletionTokens != 2048 {
		t.Fatalf("max completion tokens = %d", request.MaxCompletionTokens)
	}
}

func TestRequestValidation(t *testing.T) {
	cases := []types.Request{
		{},
		{Model: "gpt-4o", MaxTokens: -1},
		{Model: "gpt-4o", Tools: []types.Tool{{Name: "bad", InputSchema: json.RawMessage(`{`)}}},
		{Model: "gpt-4o", Messages: []types.Message{{Role: types.RoleUser, Parts: []types.Part{types.ImagePart{}}}}},
	}
	for _, request := range cases {
		if _, err := toAPIRequest(request, false); err == nil {
			t.Fatalf("request unexpectedly accepted: %+v", request)
		}
	}
}

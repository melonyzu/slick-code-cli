package ollama

import (
	"encoding/json"
	"testing"

	"github.com/melonyzu/slick-code-cli/pkg/types"
)

func TestRequestTranslatesToolHistory(t *testing.T) {
	request, err := toAPIRequest(types.Request{
		Model: "qwen3-coder:30b", MaxTokens: 1024,
		Messages: []types.Message{
			{Role: types.RoleAssistant, Parts: []types.Part{
				types.ToolCallPart{Call: types.ToolCall{ID: "ollama_call_1", Name: "git_status", Input: json.RawMessage(`{}`)}},
			}},
			{Role: types.RoleUser, Parts: []types.Part{
				types.ToolResultPart{Result: types.ToolResult{ToolCallID: "ollama_call_1", Content: "clean"}},
			}},
		},
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if request.Model != "qwen3-coder:30b" || request.Options.NumPredict != 1024 ||
		len(request.Messages) != 2 || request.Messages[1].Role != "tool" ||
		request.Messages[1].ToolName != "git_status" {
		t.Fatalf("request = %+v", request)
	}
}

func TestExactModelAndRequestValidation(t *testing.T) {
	invalid := []types.Request{
		{},
		{Model: " qwen:7b"},
		{Model: "qwen;rm:7b"},
		{Model: "qwen:7b", MaxTokens: -1},
		{Model: "qwen:7b", Tools: []types.Tool{{Name: "bad", InputSchema: json.RawMessage(`{`)}}},
	}
	for _, request := range invalid {
		if _, err := toAPIRequest(request, false); err == nil {
			t.Fatalf("request unexpectedly accepted: %+v", request)
		}
	}
}

func TestToolCapabilityGating(t *testing.T) {
	model := toModel("llama3.1:8b", []string{"completion"})
	if model.ID != "llama3.1:8b" || model.Capabilities.Has(types.CapabilityTools) ||
		!model.Capabilities.Has(types.CapabilityStreaming) {
		t.Fatalf("model = %+v", model)
	}
	if supportsChat([]string{"embedding"}) || !supportsChat(nil) {
		t.Fatal("chat capability detection failed")
	}
}

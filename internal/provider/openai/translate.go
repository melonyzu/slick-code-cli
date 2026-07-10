package openai

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/melonyzu/slick-code-cli/pkg/types"
)

type apiRequest struct {
	Model               string       `json:"model"`
	Messages            []apiMessage `json:"messages"`
	Tools               []apiTool    `json:"tools,omitempty"`
	MaxCompletionTokens int          `json:"max_completion_tokens,omitempty"`
	Stream              bool         `json:"stream,omitempty"`
	StreamOptions       *struct {
		IncludeUsage bool `json:"include_usage"`
	} `json:"stream_options,omitempty"`
}

type apiMessage struct {
	Role       string        `json:"role"`
	Content    string        `json:"content,omitempty"`
	ToolCalls  []apiToolCall `json:"tool_calls,omitempty"`
	ToolCallID string        `json:"tool_call_id,omitempty"`
}

type apiTool struct {
	Type     string      `json:"type"`
	Function apiFunction `json:"function"`
}

type apiFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
	Arguments   string          `json:"arguments,omitempty"`
}

type apiToolCall struct {
	Index    int         `json:"index,omitempty"`
	ID       string      `json:"id,omitempty"`
	Type     string      `json:"type,omitempty"`
	Function apiFunction `json:"function"`
}

type apiResponse struct {
	ID      string      `json:"id"`
	Model   string      `json:"model"`
	Choices []apiChoice `json:"choices"`
	Usage   apiUsage    `json:"usage"`
}

type apiChoice struct {
	Index        int        `json:"index"`
	Message      apiMessage `json:"message"`
	Delta        apiMessage `json:"delta"`
	FinishReason string     `json:"finish_reason"`
}

type apiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type apiModelList struct {
	Data []apiModel `json:"data"`
}

type apiModel struct {
	ID string `json:"id"`
}

func toAPIRequest(request types.Request, stream bool) (apiRequest, error) {
	if strings.TrimSpace(request.Model) == "" {
		return apiRequest{}, openAIValidation("model is required")
	}
	if request.MaxTokens < 0 {
		return apiRequest{}, openAIValidation("max tokens cannot be negative")
	}
	messages, err := toAPIMessages(request.Messages)
	if err != nil {
		return apiRequest{}, err
	}
	tools, err := toAPITools(request.Tools)
	if err != nil {
		return apiRequest{}, err
	}
	out := apiRequest{
		Model: strings.TrimSpace(request.Model), Messages: messages, Tools: tools,
		MaxCompletionTokens: request.MaxTokens, Stream: stream,
	}
	if stream {
		out.StreamOptions = &struct {
			IncludeUsage bool `json:"include_usage"`
		}{IncludeUsage: true}
	}
	return out, nil
}

func toAPIMessages(messages []types.Message) ([]apiMessage, error) {
	out := make([]apiMessage, 0, len(messages))
	for _, message := range messages {
		current := apiMessage{Role: string(message.Role)}
		flush := func() {
			if current.Content != "" || len(current.ToolCalls) > 0 {
				out = append(out, current)
				current = apiMessage{Role: string(message.Role)}
			}
		}
		for _, part := range message.Parts {
			switch value := part.(type) {
			case types.TextPart:
				current.Content += value.Text
			case types.ReasoningPart:
				// Reasoning is intentionally not sent back to OpenAI.
			case types.ToolCallPart:
				if message.Role != types.RoleAssistant {
					return nil, openAIValidation("tool calls must belong to an assistant message")
				}
				call, err := toAPIToolCall(value.Call)
				if err != nil {
					return nil, err
				}
				current.ToolCalls = append(current.ToolCalls, call)
			case types.ToolResultPart:
				flush()
				if value.Result.ToolCallID == "" {
					return nil, openAIValidation("tool result call ID is required")
				}
				content := value.Result.Content
				if value.Result.IsError {
					content = "Tool execution failed:\n" + content
				}
				out = append(out, apiMessage{Role: "tool", Content: content, ToolCallID: value.Result.ToolCallID})
			case types.ImagePart, types.FilePart:
				return nil, &types.Error{
					Kind: types.ErrorKindUnsupportedCapability, Provider: types.ProviderOpenAI,
					Message: fmt.Sprintf("message content %T is not supported", part),
				}
			default:
				return nil, &types.Error{
					Kind: types.ErrorKindUnsupportedCapability, Provider: types.ProviderOpenAI,
					Message: fmt.Sprintf("unknown message content %T", part),
				}
			}
		}
		flush()
	}
	return out, nil
}

func toAPITools(tools []types.Tool) ([]apiTool, error) {
	out := make([]apiTool, 0, len(tools))
	seen := make(map[string]bool, len(tools))
	for _, tool := range tools {
		if tool.Name == "" || seen[tool.Name] {
			return nil, openAIValidation("tool names must be non-empty and unique")
		}
		if len(tool.InputSchema) == 0 || !json.Valid(tool.InputSchema) {
			return nil, openAIValidation("tool " + tool.Name + " has invalid JSON schema")
		}
		seen[tool.Name] = true
		out = append(out, apiTool{Type: "function", Function: apiFunction{
			Name: tool.Name, Description: tool.Description, Parameters: tool.InputSchema,
		}})
	}
	return out, nil
}

func toAPIToolCall(call types.ToolCall) (apiToolCall, error) {
	if call.ID == "" || call.Name == "" {
		return apiToolCall{}, openAIValidation("tool call ID and name are required")
	}
	arguments := call.Input
	if len(arguments) == 0 {
		arguments = json.RawMessage(`{}`)
	}
	if !json.Valid(arguments) {
		return apiToolCall{}, openAIValidation("tool call " + call.ID + " has invalid JSON input")
	}
	return apiToolCall{
		ID: call.ID, Type: "function",
		Function: apiFunction{Name: call.Name, Arguments: string(arguments)},
	}, nil
}

func toResponse(response apiResponse) (types.Response, error) {
	if len(response.Choices) == 0 {
		return types.Response{}, openAIProviderError("response contained no choices", nil)
	}
	choice := response.Choices[0]
	message, err := toMessage(choice.Message)
	if err != nil {
		return types.Response{}, err
	}
	return types.Response{
		Message: message, Model: response.Model, StopReason: toStopReason(choice.FinishReason),
		Usage:    types.Usage{InputTokens: response.Usage.PromptTokens, OutputTokens: response.Usage.CompletionTokens},
		Metadata: types.Metadata{"openai_response_id": response.ID},
	}, nil
}

func toMessage(message apiMessage) (types.Message, error) {
	out := types.Message{Role: types.RoleAssistant}
	if message.Content != "" {
		out.Parts = append(out.Parts, types.TextPart{Text: message.Content})
	}
	for _, call := range message.ToolCalls {
		if call.ID == "" || call.Function.Name == "" || !json.Valid([]byte(call.Function.Arguments)) {
			return types.Message{}, openAIProviderError("response contained an invalid tool call", nil)
		}
		out.Parts = append(out.Parts, types.ToolCallPart{Call: types.ToolCall{
			ID: call.ID, Name: call.Function.Name, Input: json.RawMessage(call.Function.Arguments),
		}})
	}
	return out, nil
}

func toStopReason(reason string) types.StopReason {
	switch reason {
	case "length":
		return types.StopReasonMaxTokens
	case "tool_calls", "function_call":
		return types.StopReasonToolUse
	default:
		return types.StopReasonEndTurn
	}
}

func toModel(model apiModel) (types.Model, bool) {
	id := strings.ToLower(model.ID)
	if !isChatModel(id) {
		return types.Model{}, false
	}
	capabilities := []types.Capability{types.CapabilityChat, types.CapabilityStreaming}
	if supportsTools(id) {
		capabilities = append(capabilities, types.CapabilityTools)
	}
	if isReasoningModel(id) {
		capabilities = append(capabilities, types.CapabilityReasoning)
	}
	return types.Model{
		ID: model.ID, Name: model.ID, Provider: types.ProviderOpenAI,
		Capabilities: types.NewCapabilitySet(capabilities...),
	}, true
}

func isChatModel(id string) bool {
	for _, unsupported := range []string{
		"embedding", "transcribe", "whisper", "-tts", "-audio", "realtime", "image", "dall-e", "moderation",
	} {
		if strings.Contains(id, unsupported) {
			return false
		}
	}
	return strings.HasPrefix(id, "gpt-") || strings.HasPrefix(id, "chatgpt-") ||
		strings.HasPrefix(id, "o1") || strings.HasPrefix(id, "o3") || strings.HasPrefix(id, "o4")
}

func supportsTools(id string) bool {
	return strings.HasPrefix(id, "gpt-") || strings.HasPrefix(id, "chatgpt-") ||
		id == "o1" || strings.HasPrefix(id, "o1-20") || strings.HasPrefix(id, "o3") || strings.HasPrefix(id, "o4")
}

func isReasoningModel(id string) bool {
	return strings.HasPrefix(id, "gpt-5") || strings.HasPrefix(id, "o1") ||
		strings.HasPrefix(id, "o3") || strings.HasPrefix(id, "o4")
}

func openAIValidation(message string) error {
	return &types.Error{Kind: types.ErrorKindValidation, Provider: types.ProviderOpenAI, Message: message}
}

func openAIProviderError(message string, err error) error {
	return &types.Error{Kind: types.ErrorKindProvider, Provider: types.ProviderOpenAI, Message: message, Err: err}
}

package ollama

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/melonyzu/slick-code-cli/pkg/types"
)

type apiChatRequest struct {
	Model    string       `json:"model"`
	Messages []apiMessage `json:"messages"`
	Stream   bool         `json:"stream"`
	Tools    []apiTool    `json:"tools,omitempty"`
	Options  *apiOptions  `json:"options,omitempty"`
}

type apiOptions struct {
	NumPredict int `json:"num_predict,omitempty"`
}

type apiMessage struct {
	Role      string        `json:"role"`
	Content   string        `json:"content,omitempty"`
	Thinking  string        `json:"thinking,omitempty"`
	ToolCalls []apiToolCall `json:"tool_calls,omitempty"`
	ToolName  string        `json:"tool_name,omitempty"`
}

type apiTool struct {
	Type     string      `json:"type"`
	Function apiFunction `json:"function"`
}

type apiFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
	Arguments   json.RawMessage `json:"arguments,omitempty"`
}

type apiToolCall struct {
	ID       string      `json:"id,omitempty"`
	Function apiFunction `json:"function"`
}

type apiChatResponse struct {
	Error           string     `json:"error,omitempty"`
	Model           string     `json:"model"`
	Message         apiMessage `json:"message"`
	Done            bool       `json:"done"`
	DoneReason      string     `json:"done_reason"`
	TotalDuration   int64      `json:"total_duration"`
	LoadDuration    int64      `json:"load_duration"`
	PromptEvalCount int        `json:"prompt_eval_count"`
	EvalCount       int        `json:"eval_count"`
}

type apiTagsResponse struct {
	Models []apiTaggedModel `json:"models"`
}

type apiTaggedModel struct {
	Name  string `json:"name"`
	Model string `json:"model"`
}

func (m apiTaggedModel) exactName() string {
	if m.Name != "" {
		return m.Name
	}
	return m.Model
}

type apiShowRequest struct {
	Model string `json:"model"`
}

type apiShowResponse struct {
	Capabilities []string `json:"capabilities"`
}

func toAPIRequest(request types.Request, stream bool) (apiChatRequest, error) {
	if err := validateModelName(request.Model); err != nil {
		return apiChatRequest{}, err
	}
	if request.MaxTokens < 0 {
		return apiChatRequest{}, ollamaValidation("max tokens cannot be negative")
	}
	messages, err := toAPIMessages(request.Messages)
	if err != nil {
		return apiChatRequest{}, err
	}
	tools, err := toAPITools(request.Tools)
	if err != nil {
		return apiChatRequest{}, err
	}
	out := apiChatRequest{Model: request.Model, Messages: messages, Stream: stream, Tools: tools}
	if request.MaxTokens > 0 {
		out.Options = &apiOptions{NumPredict: request.MaxTokens}
	}
	return out, nil
}

func toAPIMessages(messages []types.Message) ([]apiMessage, error) {
	out := make([]apiMessage, 0, len(messages))
	callNames := make(map[string]string)
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
				// Reasoning is not sent back to the model.
			case types.ToolCallPart:
				if message.Role != types.RoleAssistant {
					return nil, ollamaValidation("tool calls must belong to an assistant message")
				}
				call, err := toAPIToolCall(value.Call)
				if err != nil {
					return nil, err
				}
				callNames[value.Call.ID] = value.Call.Name
				current.ToolCalls = append(current.ToolCalls, call)
			case types.ToolResultPart:
				flush()
				name := callNames[value.Result.ToolCallID]
				if value.Result.ToolCallID == "" || name == "" {
					return nil, ollamaValidation("tool result must reference a prior tool call")
				}
				content := value.Result.Content
				if value.Result.IsError {
					content = "Tool execution failed:\n" + content
				}
				out = append(out, apiMessage{Role: "tool", ToolName: name, Content: content})
			case types.ImagePart, types.FilePart:
				return nil, &types.Error{
					Kind: types.ErrorKindUnsupportedCapability, Provider: types.ProviderOllama,
					Message: fmt.Sprintf("message content %T is not supported", part),
				}
			default:
				return nil, &types.Error{
					Kind: types.ErrorKindUnsupportedCapability, Provider: types.ProviderOllama,
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
			return nil, ollamaValidation("tool names must be non-empty and unique")
		}
		if len(tool.InputSchema) == 0 || !json.Valid(tool.InputSchema) {
			return nil, ollamaValidation("tool " + tool.Name + " has invalid JSON schema")
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
		return apiToolCall{}, ollamaValidation("tool call ID and name are required")
	}
	arguments := call.Input
	if len(arguments) == 0 {
		arguments = json.RawMessage(`{}`)
	}
	if !json.Valid(arguments) {
		return apiToolCall{}, ollamaValidation("tool call " + call.ID + " has invalid JSON input")
	}
	return apiToolCall{Function: apiFunction{Name: call.Name, Arguments: arguments}}, nil
}

func toResponse(response apiChatResponse) (types.Response, error) {
	if response.Error != "" {
		return types.Response{}, ollamaProviderError(response.Error, nil)
	}
	message, err := toMessage(response.Message)
	if err != nil {
		return types.Response{}, err
	}
	stop := toStopReason(response.DoneReason, len(response.Message.ToolCalls) > 0)
	return types.Response{
		Message: message, Model: response.Model, StopReason: stop,
		Usage: types.Usage{InputTokens: response.PromptEvalCount, OutputTokens: response.EvalCount},
		Metadata: types.Metadata{
			"ollama_total_duration_ns": strconv.FormatInt(response.TotalDuration, 10),
			"ollama_load_duration_ns":  strconv.FormatInt(response.LoadDuration, 10),
		},
	}, nil
}

func toMessage(message apiMessage) (types.Message, error) {
	out := types.Message{Role: types.RoleAssistant}
	if message.Thinking != "" {
		out.Parts = append(out.Parts, types.ReasoningPart{Text: message.Thinking})
	}
	if message.Content != "" {
		out.Parts = append(out.Parts, types.TextPart{Text: message.Content})
	}
	for index, call := range message.ToolCalls {
		domainCall, err := toDomainToolCall(call, index)
		if err != nil {
			return types.Message{}, err
		}
		out.Parts = append(out.Parts, types.ToolCallPart{Call: domainCall})
	}
	return out, nil
}

func toDomainToolCall(call apiToolCall, index int) (types.ToolCall, error) {
	if call.Function.Name == "" {
		return types.ToolCall{}, ollamaProviderError("response contained a tool call without a name", nil)
	}
	arguments := call.Function.Arguments
	if len(arguments) == 0 {
		arguments = json.RawMessage(`{}`)
	}
	if !json.Valid(arguments) {
		return types.ToolCall{}, ollamaProviderError("response contained invalid tool arguments", nil)
	}
	id := call.ID
	if id == "" {
		id = "ollama_call_" + strconv.Itoa(index+1)
	}
	return types.ToolCall{ID: id, Name: call.Function.Name, Input: arguments}, nil
}

func toStopReason(reason string, hasTools bool) types.StopReason {
	if hasTools {
		return types.StopReasonToolUse
	}
	if reason == "length" {
		return types.StopReasonMaxTokens
	}
	return types.StopReasonEndTurn
}

func toModel(name string, capabilities []string) types.Model {
	set := []types.Capability{types.CapabilityChat, types.CapabilityStreaming}
	for _, capability := range capabilities {
		switch capability {
		case "tools":
			set = append(set, types.CapabilityTools)
		case "thinking":
			set = append(set, types.CapabilityReasoning)
		}
	}
	return types.Model{
		ID: name, Name: name, Provider: types.ProviderOllama,
		Capabilities: types.NewCapabilitySet(set...),
	}
}

func supportsChat(capabilities []string) bool {
	if len(capabilities) == 0 {
		return true
	}
	for _, capability := range capabilities {
		if capability == "completion" {
			return true
		}
	}
	return false
}

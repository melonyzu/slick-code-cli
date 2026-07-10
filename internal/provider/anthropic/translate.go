package anthropic

import (
	"fmt"

	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// defaultMaxTokens applies when the request does not cap the response;
// the Messages API requires an explicit limit.
const defaultMaxTokens = 4096

// apiRequest is the Messages API request shape.
type apiRequest struct {
	Model     string       `json:"model"`
	MaxTokens int          `json:"max_tokens"`
	System    string       `json:"system,omitempty"`
	Messages  []apiMessage `json:"messages"`
	Stream    bool         `json:"stream,omitempty"`
}

type apiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// apiResponse is the Messages API response shape.
type apiResponse struct {
	Model      string            `json:"model"`
	Content    []apiContentBlock `json:"content"`
	StopReason string            `json:"stop_reason"`
	Usage      apiUsage          `json:"usage"`
}

type apiContentBlock struct {
	Type     string `json:"type"`
	Text     string `json:"text"`
	Thinking string `json:"thinking"`
}

type apiUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// apiModelPage is one page of the model listing.
type apiModelPage struct {
	Data    []apiModel `json:"data"`
	HasMore bool       `json:"has_more"`
}

type apiModel struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

// toAPIRequest translates a domain request into the Messages API shape.
// System messages become the request's system prompt; only text content
// is supported in this version.
func toAPIRequest(req types.Request, stream bool) (apiRequest, error) {
	out := apiRequest{
		Model:     req.Model,
		MaxTokens: req.MaxTokens,
		Stream:    stream,
	}
	if out.MaxTokens == 0 {
		out.MaxTokens = defaultMaxTokens
	}

	for _, msg := range req.Messages {
		if err := textOnly(msg); err != nil {
			return apiRequest{}, err
		}

		text := msg.Text()
		if text == "" {
			continue
		}

		if msg.Role == types.RoleSystem {
			if out.System != "" {
				out.System += "\n\n"
			}
			out.System += text
			continue
		}
		out.Messages = append(out.Messages, apiMessage{Role: string(msg.Role), Content: text})
	}

	return out, nil
}

// textOnly rejects message content this version cannot send. Reasoning
// parts are tolerated (and skipped) because assistant turns legitimately
// contain them.
func textOnly(msg types.Message) error {
	for _, part := range msg.Parts {
		switch part.(type) {
		case types.TextPart, types.ReasoningPart:
		default:
			return &types.Error{
				Kind:     types.ErrorKindUnsupportedCapability,
				Provider: types.ProviderAnthropic,
				Message:  fmt.Sprintf("message content %T is not supported yet", part),
			}
		}
	}
	return nil
}

// toResponse translates a Messages API response into the domain model.
func toResponse(r apiResponse) types.Response {
	msg := types.Message{Role: types.RoleAssistant}
	for _, block := range r.Content {
		switch block.Type {
		case "text":
			msg.Parts = append(msg.Parts, types.TextPart{Text: block.Text})
		case "thinking":
			msg.Parts = append(msg.Parts, types.ReasoningPart{Text: block.Thinking})
		}
	}

	return types.Response{
		Message:    msg,
		Model:      r.Model,
		StopReason: toStopReason(r.StopReason),
		Usage: types.Usage{
			InputTokens:  r.Usage.InputTokens,
			OutputTokens: r.Usage.OutputTokens,
		},
	}
}

// toStopReason maps the API's stop reasons onto the domain's.
func toStopReason(s string) types.StopReason {
	switch s {
	case "max_tokens":
		return types.StopReasonMaxTokens
	case "tool_use":
		return types.StopReasonToolUse
	default:
		// end_turn, stop_sequence, and anything future all mean the
		// model finished speaking.
		return types.StopReasonEndTurn
	}
}

// toModel translates a listed model into the domain model. Capabilities
// reflect what this package implements, not everything the model could
// do.
func toModel(m apiModel) types.Model {
	return types.Model{
		ID:           m.ID,
		Name:         m.DisplayName,
		Provider:     types.ProviderAnthropic,
		Capabilities: types.NewCapabilitySet(types.CapabilityChat, types.CapabilityStreaming),
	}
}

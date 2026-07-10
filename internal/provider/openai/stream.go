package openai

import (
	"bufio"
	"context"
	"encoding/json"
	"iter"
	"strings"

	"github.com/melonyzu/slick-code-cli/pkg/types"
)

const sseMaxLineSize = 1 << 20

type streamChunk struct {
	ID      string      `json:"id"`
	Model   string      `json:"model"`
	Choices []apiChoice `json:"choices"`
	Usage   *apiUsage   `json:"usage"`
	Error   *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

type pendingToolCall struct {
	id        string
	name      string
	arguments strings.Builder
}

// Stream implements provider.Streamer over OpenAI's SSE chat stream.
func (p *Provider) Stream(ctx context.Context, request types.Request) (iter.Seq2[types.StreamEvent, error], error) {
	c, err := p.activeClient()
	if err != nil {
		return nil, err
	}
	apiRequest, err := toAPIRequest(request, true)
	if err != nil {
		return nil, err
	}
	response, err := c.chatStream(ctx, apiRequest)
	if err != nil {
		return nil, err
	}

	return func(yield func(types.StreamEvent, error) bool) {
		defer response.Body.Close()
		scanner := bufio.NewScanner(response.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), sseMaxLineSize)

		var (
			text       strings.Builder
			model      = request.Model
			responseID string
			stopReason = types.StopReasonEndTurn
			usage      types.Usage
			calls      = make(map[int]*pendingToolCall)
		)

		finish := func() bool {
			message := types.Message{Role: types.RoleAssistant}
			if text.Len() > 0 {
				message.Parts = append(message.Parts, types.TextPart{Text: text.String()})
			}
			for index := 0; index < len(calls); index++ {
				call, ok := calls[index]
				if !ok || call.id == "" || call.name == "" || !json.Valid([]byte(call.arguments.String())) {
					yield(nil, openAIProviderError("stream contained an invalid tool call", nil))
					return false
				}
				domainCall := types.ToolCall{
					ID: call.id, Name: call.name, Input: json.RawMessage(call.arguments.String()),
				}
				message.Parts = append(message.Parts, types.ToolCallPart{Call: domainCall})
				if !yield(types.ToolCallEvent{Call: domainCall}, nil) {
					return false
				}
			}
			return yield(types.DoneEvent{Response: types.Response{
				Message: message, Model: model, StopReason: stopReason, Usage: usage,
				Metadata: types.Metadata{"openai_response_id": responseID},
			}}, nil)
		}

		for scanner.Scan() {
			line := scanner.Text()
			data, ok := strings.CutPrefix(line, "data:")
			if !ok {
				continue
			}
			data = strings.TrimSpace(data)
			if data == "[DONE]" {
				finish()
				return
			}
			var chunk streamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				yield(nil, openAIProviderError("decode stream event", err))
				return
			}
			if chunk.Error != nil {
				yield(nil, &types.Error{
					Kind: kindForAPIType(chunk.Error.Type), Provider: types.ProviderOpenAI, Message: chunk.Error.Message,
				})
				return
			}
			if chunk.ID != "" {
				responseID = chunk.ID
			}
			if chunk.Model != "" {
				model = chunk.Model
			}
			if chunk.Usage != nil {
				usage.InputTokens = chunk.Usage.PromptTokens
				usage.OutputTokens = chunk.Usage.CompletionTokens
			}
			for _, choice := range chunk.Choices {
				if choice.Index != 0 {
					continue
				}
				if choice.Delta.Content != "" {
					text.WriteString(choice.Delta.Content)
					if !yield(types.TextEvent{Text: choice.Delta.Content}, nil) {
						return
					}
				}
				for _, delta := range choice.Delta.ToolCalls {
					call := calls[delta.Index]
					if call == nil {
						call = &pendingToolCall{}
						calls[delta.Index] = call
					}
					if delta.ID != "" {
						call.id = delta.ID
					}
					if delta.Function.Name != "" {
						call.name += delta.Function.Name
					}
					call.arguments.WriteString(delta.Function.Arguments)
				}
				if choice.FinishReason != "" {
					stopReason = toStopReason(choice.FinishReason)
				}
			}
		}

		if err := scanner.Err(); err != nil {
			yield(nil, translateTransportError(err))
			return
		}
		if err := ctx.Err(); err != nil {
			yield(nil, translateTransportError(err))
			return
		}
		yield(nil, openAIProviderError("stream ended without [DONE]", nil))
	}, nil
}

func kindForAPIType(errorType string) types.ErrorKind {
	switch errorType {
	case "authentication_error", "invalid_api_key":
		return types.ErrorKindAuthentication
	case "rate_limit_error", "insufficient_quota":
		return types.ErrorKindRateLimit
	case "invalid_request_error":
		return types.ErrorKindValidation
	default:
		return types.ErrorKindProvider
	}
}

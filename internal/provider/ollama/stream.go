package ollama

import (
	"bufio"
	"context"
	"encoding/json"
	"iter"
	"strconv"
	"strings"

	"github.com/melonyzu/slick-code-cli/pkg/types"
)

const streamMaxLineSize = 1 << 20

// Stream implements provider.Streamer over Ollama's newline-delimited JSON
// chat stream.
func (p *Provider) Stream(ctx context.Context, request types.Request) (iter.Seq2[types.StreamEvent, error], error) {
	if err := p.requireActive(); err != nil {
		return nil, err
	}
	apiRequest, err := toAPIRequest(request, true)
	if err != nil {
		return nil, err
	}
	if supported, known := p.modelSupportsTools(request.Model); known && !supported {
		apiRequest.Tools = nil
	}
	response, err := p.client.chatStream(ctx, apiRequest)
	if err != nil {
		return nil, err
	}

	return func(yield func(types.StreamEvent, error) bool) {
		defer response.Body.Close()
		scanner := bufio.NewScanner(response.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), streamMaxLineSize)
		var text, reasoning strings.Builder
		var calls []types.ToolCall
		model := request.Model

		for scanner.Scan() {
			var chunk apiChatResponse
			if err := json.Unmarshal(scanner.Bytes(), &chunk); err != nil {
				yield(nil, ollamaProviderError("decode stream chunk", err))
				return
			}
			if chunk.Error != "" {
				if strings.Contains(strings.ToLower(chunk.Error), "not found") {
					yield(nil, modelNotFound(request.Model))
				} else {
					yield(nil, ollamaProviderError(chunk.Error, nil))
				}
				return
			}
			if chunk.Model != "" {
				model = chunk.Model
			}
			if chunk.Message.Thinking != "" {
				reasoning.WriteString(chunk.Message.Thinking)
				if !yield(types.ReasoningEvent{Text: chunk.Message.Thinking}, nil) {
					return
				}
			}
			if chunk.Message.Content != "" {
				text.WriteString(chunk.Message.Content)
				if !yield(types.TextEvent{Text: chunk.Message.Content}, nil) {
					return
				}
			}
			for _, call := range chunk.Message.ToolCalls {
				domainCall, err := toDomainToolCall(call, len(calls))
				if err != nil {
					yield(nil, err)
					return
				}
				calls = append(calls, domainCall)
				if !yield(types.ToolCallEvent{Call: domainCall}, nil) {
					return
				}
			}
			if !chunk.Done {
				continue
			}
			message := types.Message{Role: types.RoleAssistant}
			if reasoning.Len() > 0 {
				message.Parts = append(message.Parts, types.ReasoningPart{Text: reasoning.String()})
			}
			if text.Len() > 0 {
				message.Parts = append(message.Parts, types.TextPart{Text: text.String()})
			}
			for _, call := range calls {
				message.Parts = append(message.Parts, types.ToolCallPart{Call: call})
			}
			yield(types.DoneEvent{Response: types.Response{
				Message: message, Model: model, StopReason: toStopReason(chunk.DoneReason, len(calls) > 0),
				Usage: types.Usage{InputTokens: chunk.PromptEvalCount, OutputTokens: chunk.EvalCount},
				Metadata: types.Metadata{
					"ollama_total_duration_ns": strconv.FormatInt(chunk.TotalDuration, 10),
					"ollama_load_duration_ns":  strconv.FormatInt(chunk.LoadDuration, 10),
				},
			}}, nil)
			return
		}
		if err := scanner.Err(); err != nil {
			yield(nil, translateTransportError(err, p.endpoint))
			return
		}
		if err := ctx.Err(); err != nil {
			yield(nil, translateTransportError(err, p.endpoint))
			return
		}
		yield(nil, ollamaProviderError("stream ended before Ollama reported done", nil))
	}, nil
}

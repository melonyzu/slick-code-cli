package anthropic

import (
	"bufio"
	"context"
	"encoding/json"
	"iter"
	"strings"

	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// sseMaxLineSize bounds a single server-sent-event line; delta payloads
// are small but final message events can carry sizable text.
const sseMaxLineSize = 1 << 20

// sseEvent is the union of streaming event payloads this package
// consumes. Fields are populated depending on the event type.
type sseEvent struct {
	Type    string `json:"type"`
	Message *struct {
		Model string   `json:"model"`
		Usage apiUsage `json:"usage"`
	} `json:"message"` // message_start
	Delta *struct {
		Type       string `json:"type"`
		Text       string `json:"text"`
		Thinking   string `json:"thinking"`
		StopReason string `json:"stop_reason"`
	} `json:"delta"` // content_block_delta, message_delta
	Usage *apiUsage `json:"usage"` // message_delta
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"` // error
}

// Stream implements provider.Streamer over the Messages API's
// server-sent events, translating them into domain stream events and
// assembling the final Response delivered with DoneEvent.
func (p *Provider) Stream(ctx context.Context, req types.Request) (iter.Seq2[types.StreamEvent, error], error) {
	c, err := p.activeClient()
	if err != nil {
		return nil, err
	}

	apiReq, err := toAPIRequest(req, true)
	if err != nil {
		return nil, err
	}

	resp, err := c.messagesStream(ctx, apiReq)
	if err != nil {
		return nil, err
	}

	return func(yield func(types.StreamEvent, error) bool) {
		defer resp.Body.Close()

		var (
			text       strings.Builder
			reasoning  strings.Builder
			model      = req.Model
			usage      types.Usage
			stopReason types.StopReason = types.StopReasonEndTurn
		)

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), sseMaxLineSize)

		for scanner.Scan() {
			data, ok := strings.CutPrefix(scanner.Text(), "data: ")
			if !ok {
				continue // event: lines, comments, blank separators
			}

			var ev sseEvent
			if err := json.Unmarshal([]byte(data), &ev); err != nil {
				yield(nil, types.WrapError(types.ErrorKindProvider,
					"anthropic: decode stream event", err))
				return
			}

			switch ev.Type {
			case "message_start":
				if ev.Message != nil {
					model = ev.Message.Model
					usage.InputTokens = ev.Message.Usage.InputTokens
				}

			case "content_block_delta":
				if ev.Delta == nil {
					continue
				}
				switch ev.Delta.Type {
				case "text_delta":
					text.WriteString(ev.Delta.Text)
					if !yield(types.TextEvent{Text: ev.Delta.Text}, nil) {
						return
					}
				case "thinking_delta":
					reasoning.WriteString(ev.Delta.Thinking)
					if !yield(types.ReasoningEvent{Text: ev.Delta.Thinking}, nil) {
						return
					}
				}

			case "message_delta":
				if ev.Delta != nil && ev.Delta.StopReason != "" {
					stopReason = toStopReason(ev.Delta.StopReason)
				}
				if ev.Usage != nil {
					usage.OutputTokens = ev.Usage.OutputTokens
				}

			case "message_stop":
				yield(types.DoneEvent{Response: assembleResponse(
					model, text.String(), reasoning.String(), stopReason, usage)}, nil)
				return

			case "error":
				message := "stream failed"
				if ev.Error != nil {
					message = ev.Error.Message
				}
				yield(nil, &types.Error{
					Kind:     types.ErrorKindProvider,
					Provider: types.ProviderAnthropic,
					Message:  message,
				})
				return
			}
		}

		// The stream ended without message_stop: canceled or torn down.
		err := scanner.Err()
		if err == nil {
			err = ctx.Err()
		}
		if err == nil {
			err = types.NewError(types.ErrorKindProvider, "anthropic: stream ended unexpectedly")
		}
		yield(nil, err)
	}, nil
}

// assembleResponse builds the final Response from accumulated stream
// state.
func assembleResponse(model, text, reasoning string, stop types.StopReason, usage types.Usage) types.Response {
	msg := types.Message{Role: types.RoleAssistant}
	if reasoning != "" {
		msg.Parts = append(msg.Parts, types.ReasoningPart{Text: reasoning})
	}
	if text != "" {
		msg.Parts = append(msg.Parts, types.TextPart{Text: text})
	}

	return types.Response{
		Message:    msg,
		Model:      model,
		StopReason: stop,
		Usage:      usage,
	}
}

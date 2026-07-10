package repl

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/melonyzu/slick-code-cli/internal/provider"
	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// Messages delivered from an in-flight assistant turn to the UI loop.
type (
	// streamTextMsg carries an answer or reasoning fragment.
	streamTextMsg struct {
		text      string
		reasoning bool
	}

	// streamDoneMsg terminates a turn with the assembled response.
	streamDoneMsg struct {
		response types.Response
	}

	// streamErrMsg terminates a turn with a failure.
	streamErrMsg struct {
		err error
	}
)

// turn is one in-flight assistant response: a producer goroutine
// pushing messages that the UI loop drains one per Update via poll.
type turn struct {
	cancel context.CancelFunc
	events chan tea.Msg
	buf    string // answer text accumulated so far, for live display
}

// startTurn launches the provider call for req and returns the turn
// plus the command that polls its first message. Providers implementing
// Streamer stream fragment by fragment; otherwise Completer answers in
// one piece; the UI consumes both through the same messages.
func startTurn(ctx context.Context, p provider.Provider, req types.Request) (*turn, tea.Cmd) {
	ctx, cancel := context.WithCancel(ctx)
	t := &turn{cancel: cancel, events: make(chan tea.Msg, 32)}

	go t.produce(ctx, p, req)

	return t, t.poll()
}

// poll returns a command yielding the turn's next message.
func (t *turn) poll() tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-t.events
		if !ok {
			return nil
		}
		return msg
	}
}

// produce runs the provider exchange and always finishes with exactly
// one terminal message (done or error).
func (t *turn) produce(ctx context.Context, p provider.Provider, req types.Request) {
	defer close(t.events)

	switch impl := p.(type) {
	case provider.Streamer:
		t.produceStream(ctx, impl, req)

	case provider.Completer:
		resp, err := impl.Complete(ctx, req)
		if err != nil {
			t.events <- streamErrMsg{err}
			return
		}
		t.events <- streamDoneMsg{*resp}

	default:
		t.events <- streamErrMsg{&types.Error{
			Kind:     types.ErrorKindUnsupportedCapability,
			Provider: p.Name(),
			Message:  "provider supports neither streaming nor completion",
		}}
	}
}

// produceStream forwards stream fragments as UI messages until the
// stream completes or fails.
func (t *turn) produceStream(ctx context.Context, s provider.Streamer, req types.Request) {
	seq, err := s.Stream(ctx, req)
	if err != nil {
		t.events <- streamErrMsg{err}
		return
	}

	for ev, err := range seq {
		if err != nil {
			t.events <- streamErrMsg{err}
			return
		}
		switch e := ev.(type) {
		case types.TextEvent:
			t.events <- streamTextMsg{text: e.Text}
		case types.ReasoningEvent:
			t.events <- streamTextMsg{text: e.Text, reasoning: true}
		case types.DoneEvent:
			t.events <- streamDoneMsg{e.Response}
			return
		}
	}
	t.events <- streamErrMsg{types.NewError(types.ErrorKindProvider,
		"stream ended without completing")}
}

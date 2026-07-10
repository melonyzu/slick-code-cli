package repl

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/melonyzu/slick-code-cli/internal/core"
	"github.com/melonyzu/slick-code-cli/internal/provider"
	"github.com/melonyzu/slick-code-cli/internal/session"
	"github.com/melonyzu/slick-code-cli/internal/terminal"
	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// inputHeight is how many terminal rows the input area occupies.
const inputHeight = 3

// model is the interactive assistant's Bubble Tea model.
type model struct {
	ctx      context.Context
	app      *core.App
	prov     provider.Provider
	provName types.Provider
	modelID  string

	sess     *session.Session
	commands []slashCommand

	ta      textarea.Model
	vp      viewport.Model
	render  *renderer
	entries []string // rendered transcript blocks
	active  *turn    // in-flight assistant response, nil when idle

	history []string // submitted inputs, for ctrl+p/ctrl+n recall
	histPos int

	width, height int
	ready         bool
	quitting      bool
}

// newModel assembles the UI for an active provider.
func newModel(ctx context.Context, app *core.App, p provider.Provider, modelID string) *model {
	ta := textarea.New()
	ta.Placeholder = "Ask anything — /help for commands"
	ta.Prompt = "┃ "
	ta.SetHeight(inputHeight - 1)
	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline.SetKeys("ctrl+j")
	ta.Focus()

	return &model{
		ctx:      ctx,
		app:      app,
		prov:     p,
		provName: p.Name(),
		modelID:  modelID,
		sess:     session.New(),
		commands: commands(),
		ta:       ta,
		render:   newRenderer(80),
	}
}

// Init implements tea.Model.
func (m *model) Init() tea.Cmd {
	return textarea.Blink
}

// Update implements tea.Model.
func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m, m.resize(msg)

	case tea.KeyMsg:
		return m.updateKey(msg)

	case streamTextMsg:
		if m.active == nil {
			return m, nil
		}
		if !msg.reasoning {
			m.active.buf += msg.text
		}
		m.refreshViewport()
		return m, m.active.poll()

	case streamDoneMsg:
		return m, m.finishTurn(msg.response)

	case streamErrMsg:
		m.abortTurn(msg.err)
		return m, nil

	case infoMsg:
		if msg.isErr {
			m.appendError(msg.text)
		} else {
			m.appendInfo(msg.text)
		}
		return m, nil
	}

	return m, m.updateComponents(msg)
}

// updateKey handles keyboard input.
func (m *model) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		switch {
		case m.active != nil:
			m.active.cancel()
			return m, nil
		case m.ta.Value() != "":
			m.ta.Reset()
			return m, nil
		default:
			m.quitting = true
			return m, tea.Quit
		}

	case "ctrl+d":
		if m.ta.Value() == "" {
			m.quitting = true
			return m, tea.Quit
		}

	case "ctrl+p":
		m.recallHistory(-1)
		return m, nil

	case "ctrl+n":
		m.recallHistory(+1)
		return m, nil

	case "enter":
		return m, m.submit()
	}

	return m, m.updateComponents(msg)
}

// submit handles Enter: slash commands dispatch, anything else becomes
// a chat turn. During an in-flight response only /-commands run, so
// /exit always works.
func (m *model) submit() tea.Cmd {
	input := strings.TrimSpace(m.ta.Value())
	if input == "" {
		return nil
	}

	m.rememberInput(input)
	m.ta.Reset()

	if strings.HasPrefix(input, "/") {
		return m.dispatch(input)
	}

	if m.active != nil {
		m.appendInfo("A response is still streaming — press ctrl+c to interrupt it first.")
		return nil
	}
	return m.startChat(input)
}

// startChat begins an assistant turn for the user's prompt.
func (m *model) startChat(prompt string) tea.Cmd {
	m.sess.Append(types.NewTextMessage(types.RoleUser, prompt))
	m.entries = append(m.entries, m.render.user(prompt))

	if _, err := m.app.Context.Refresh(m.ctx); err != nil {
		m.app.Logger.Warn("project context refresh failed; using last snapshot", "error", err)
	}
	snapshot := m.app.Context.Snapshot()
	messages := make([]types.Message, 0, len(m.sess.Messages())+1)
	if snapshot.Text != "" {
		messages = append(messages, types.NewTextMessage(types.RoleSystem, snapshot.Text))
	}
	messages = append(messages, m.sess.Messages()...)
	req := types.Request{
		Model:    m.modelID,
		Messages: messages,
		Tools:    m.app.Tools.Registry().Definitions(),
	}

	t, poll := startTurn(m.ctx, m.prov, req)
	m.active = t
	m.refreshViewport()
	return poll
}

// finishTurn lands a completed response in the transcript and session.
func (m *model) finishTurn(resp types.Response) tea.Cmd {
	m.active = nil
	m.sess.Append(resp.Message)

	m.entries = append(m.entries, m.render.markdown(resp.Message.Text()))
	m.entries = append(m.entries, m.render.info(fmt.Sprintf(
		"%s · %d in / %d out tokens", resp.Model,
		resp.Usage.InputTokens, resp.Usage.OutputTokens)))
	m.refreshViewport()
	return nil
}

// abortTurn lands a failed or interrupted turn.
func (m *model) abortTurn(err error) {
	partial := ""
	if m.active != nil {
		partial = m.active.buf
	}
	m.active = nil

	// Keep whatever streamed before the interruption visible.
	if partial != "" {
		m.entries = append(m.entries, m.render.markdown(partial))
	}

	switch {
	case err == nil || errors.Is(err, context.Canceled):
		m.entries = append(m.entries, m.render.info("Interrupted."))
	default:
		m.entries = append(m.entries, m.render.errorLine(err.Error()))
	}
	m.refreshViewport()
}

// resize adapts the layout to the terminal size.
func (m *model) resize(msg tea.WindowSizeMsg) tea.Cmd {
	m.width, m.height = msg.Width, msg.Height
	m.render = newRenderer(msg.Width)

	vpHeight := max(1, msg.Height-inputHeight-2) // header + footer
	if !m.ready {
		m.vp = viewport.New(msg.Width, vpHeight)
		m.ready = true
	} else {
		m.vp.Width = msg.Width
		m.vp.Height = vpHeight
	}
	m.ta.SetWidth(msg.Width)
	m.refreshViewport()
	return nil
}

// updateComponents forwards a message to the textarea and viewport.
func (m *model) updateComponents(msg tea.Msg) tea.Cmd {
	var taCmd, vpCmd tea.Cmd
	m.ta, taCmd = m.ta.Update(msg)
	m.vp, vpCmd = m.vp.Update(msg)
	return tea.Batch(taCmd, vpCmd)
}

// refreshViewport rebuilds the transcript view and follows the tail.
func (m *model) refreshViewport() {
	if !m.ready {
		return
	}

	content := strings.Join(m.entries, "\n")
	if m.active != nil {
		live := m.active.buf
		if live == "" {
			live = "…"
		}
		content += "\n" + live
	}
	m.vp.SetContent(content)
	m.vp.GotoBottom()
}

// appendInfo adds an informational entry to the transcript.
func (m *model) appendInfo(text string) {
	m.entries = append(m.entries, m.render.info(text))
	m.refreshViewport()
}

// appendError adds an error entry to the transcript.
func (m *model) appendError(text string) {
	m.entries = append(m.entries, m.render.errorLine(text))
	m.refreshViewport()
}

// rememberInput records a submitted input for history recall.
func (m *model) rememberInput(input string) {
	m.history = append(m.history, input)
	m.histPos = len(m.history)
}

// recallHistory moves through submitted inputs; step is -1 for older,
// +1 for newer.
func (m *model) recallHistory(step int) {
	if len(m.history) == 0 {
		return
	}

	m.histPos += step
	switch {
	case m.histPos < 0:
		m.histPos = 0
	case m.histPos >= len(m.history):
		m.histPos = len(m.history)
		m.ta.Reset()
		return
	}
	m.ta.SetValue(m.history[m.histPos])
}

// View implements tea.Model.
func (m *model) View() string {
	if m.quitting {
		return ""
	}
	if !m.ready {
		return "starting…"
	}

	header := terminal.Title.Render("slick code") + terminal.Muted.Render(
		fmt.Sprintf("  %s · %s", m.provName, m.modelID))
	footer := terminal.Muted.Render("enter send · ctrl+j newline · ctrl+c interrupt · /help")

	return header + "\n" + m.vp.View() + "\n" + m.ta.View() + "\n" + footer
}

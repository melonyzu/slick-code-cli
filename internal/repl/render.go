package repl

import (
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/melonyzu/slick-code-cli/internal/terminal"
)

// renderer converts markdown to terminal output and falls back to raw
// text when rendering is unavailable.
type renderer struct {
	glam *glamour.TermRenderer
}

// newRenderer returns a renderer wrapped to the given width.
func newRenderer(width int) *renderer {
	glam, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(max(20, width-2)),
	)
	if err != nil {
		return &renderer{}
	}
	return &renderer{glam: glam}
}

// markdown renders markdown for the terminal.
func (r *renderer) markdown(md string) string {
	if r.glam == nil {
		return md
	}
	out, err := r.glam.Render(md)
	if err != nil {
		return md
	}
	return strings.TrimRight(out, "\n")
}

// user renders a submitted user message.
func (r *renderer) user(text string) string {
	return terminal.Accent.Render("You") + "\n" + text
}

// assistant renders an assistant message.
func (r *renderer) assistant(text string) string {
	body := strings.TrimSpace(r.markdown(text))
	if body == "" {
		body = terminal.Muted.Render("Thinking…")
	}
	return terminal.Title.Render("Assistant") + "\n" + body
}

// info renders an informational entry.
func (r *renderer) info(text string) string {
	return terminal.Muted.Render(text)
}

// errorLine renders an error entry.
func (r *renderer) errorLine(text string) string {
	return terminal.Error.Render("✗ " + text)
}

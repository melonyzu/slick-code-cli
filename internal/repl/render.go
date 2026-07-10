package repl

import (
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	"github.com/melonyzu/slick-code-cli/internal/terminal"
)

// Local styles for transcript entries; shared base styles come from
// internal/terminal.
var (
	userPrefixStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	infoStyle       = terminal.Muted
	errorStyle      = terminal.Error
)

// renderer converts markdown to styled terminal output, falling back to
// the raw text when rendering is unavailable (e.g. dumb terminals).
type renderer struct {
	glam *glamour.TermRenderer
}

// newRenderer returns a renderer wrapping to the given width.
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

// markdown renders md for the terminal.
func (r *renderer) markdown(md string) string {
	if r.glam == nil {
		return md
	}
	out, err := r.glam.Render(md)
	if err != nil {
		return md
	}
	return strings.TrimRight(out, "\n") + "\n"
}

// user renders a submitted user message.
func (r *renderer) user(text string) string {
	return userPrefixStyle.Render("you ❯ ") + text + "\n"
}

// info renders an informational (slash command output) entry.
func (r *renderer) info(text string) string {
	return infoStyle.Render(text) + "\n"
}

// errorLine renders an error entry.
func (r *renderer) errorLine(text string) string {
	return errorStyle.Render("✗ "+text) + "\n"
}

// Package repl implements Slick Code's interactive assistant: a
// terminal UI with streaming responses, markdown rendering, input
// history, and slash commands, built on Bubble Tea. It is
// provider-agnostic — the active provider arrives through the
// provider contracts and everything else through *core.App.
package repl

import (
	"context"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/melonyzu/slick-code-cli/internal/core"
	"github.com/melonyzu/slick-code-cli/internal/provider"
)

// Run starts the interactive assistant against an already-activated
// provider and blocks until the user leaves or ctx is canceled.
func Run(ctx context.Context, app *core.App, p provider.Provider, modelID string) error {
	m := newModel(ctx, app, p, modelID)

	opts := []tea.ProgramOption{
		tea.WithContext(ctx),
		tea.WithOutput(app.Terminal.Out),
	}
	// On a real terminal Bubble Tea manages raw input itself; anywhere
	// else (tests, pipes) it must share the terminal's buffered reader
	// so bytes consumed by earlier prompts aren't lost.
	if f, ok := app.Terminal.In.(*os.File); ok {
		opts = append(opts, tea.WithInput(f))
	} else {
		opts = append(opts, tea.WithInput(app.Terminal.Reader()))
	}

	_, err := tea.NewProgram(m, opts...).Run()
	if err == tea.ErrProgramKilled || err == context.Canceled {
		return nil
	}
	return err
}

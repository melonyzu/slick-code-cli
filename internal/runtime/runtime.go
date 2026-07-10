// Package runtime owns the CLI's process lifecycle: given an assembled
// App, it builds the command tree and executes it, installing a graceful
// shutdown handler so that future long-running commands can observe
// cancellation instead of being killed outright.
package runtime

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/melonyzu/slick-code-cli/internal/command"
	"github.com/melonyzu/slick-code-cli/internal/core"
)

// Runtime executes the CLI for a single process invocation.
type Runtime struct {
	app *core.App
}

// New returns a Runtime that executes commands against app.
func New(app *core.App) *Runtime {
	return &Runtime{app: app}
}

// Run builds the command tree and executes it against ctx. ctx is
// wrapped so that an interrupt or termination signal cancels it, giving
// commands a chance to shut down gracefully instead of being killed
// outright.
func (r *Runtime) Run(ctx context.Context) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	root := command.NewRootCommand(r.app)
	return root.ExecuteContext(ctx)
}

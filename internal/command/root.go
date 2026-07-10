// Package command implements Slick Code's command-line interface using
// Cobra. Commands are thin: they parse flags and delegate to internal/core.
package command

import (
	"github.com/spf13/cobra"

	"github.com/melonyzu/slick-code-cli/internal/core"
)

// NewRootCommand returns the root `slickcode` command with its
// subcommands attached, wired to app's terminal for input/output.
// Invoked with no subcommand, it launches the interactive assistant —
// running first-time setup and authentication automatically as needed.
func NewRootCommand(app *core.App) *cobra.Command {
	root := &cobra.Command{
		Use:           "slickcode",
		Short:         "A unified CLI for AI coding providers",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInteractive(cmd.Context(), app)
		},
	}

	root.SetIn(app.Terminal.In)
	root.SetOut(app.Terminal.Out)
	root.SetErr(app.Terminal.Err)

	root.AddCommand(
		newAuthCommand(app),
		newVersionCommand(),
	)

	return root
}

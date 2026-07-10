package command

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/melonyzu/slick-code-cli/pkg/version"
)

// newVersionCommand returns the `slickcode version` command.
func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the slickcode version",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), version.String())
			return err
		},
	}
}

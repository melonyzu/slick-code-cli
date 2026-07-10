package command

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/melonyzu/slick-code-cli/internal/auth"
	"github.com/melonyzu/slick-code-cli/internal/core"
	"github.com/melonyzu/slick-code-cli/internal/provider"
	"github.com/melonyzu/slick-code-cli/internal/terminal"
	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// newAuthCommand returns the `slickcode auth` command group.
func newAuthCommand(app *core.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage provider authentication",
	}

	cmd.AddCommand(
		newAuthLoginCommand(app),
		newAuthLogoutCommand(app),
		newAuthStatusCommand(app),
	)

	return cmd
}

// newAuthLoginCommand returns the `slickcode auth login` command.
func newAuthLoginCommand(app *core.App) *cobra.Command {
	var methodFlag string

	cmd := &cobra.Command{
		Use:   "login <provider>",
		Short: "Log in to a provider",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := types.Provider(args[0])

			authn, err := provider.Capability[provider.Authenticator](app.Providers, name)
			if err != nil {
				return err
			}

			method, err := chooseMethod(authn.AuthMethods(), methodFlag)
			if err != nil {
				return err
			}

			flow, err := authn.NewFlow(method)
			if err != nil {
				return err
			}

			cred, err := app.Auth.Login(cmd.Context(), name, flow, app.Terminal)
			if err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), terminal.Success.Render(
				fmt.Sprintf("Logged in to %s (%s)", name, cred.Method)))
			return nil
		},
	}

	cmd.Flags().StringVar(&methodFlag, "method", "",
		"authentication method to use (api_key, browser_oauth, device_code, none)")

	return cmd
}

// newAuthLogoutCommand returns the `slickcode auth logout` command.
func newAuthLogoutCommand(app *core.App) *cobra.Command {
	return &cobra.Command{
		Use:   "logout <provider>",
		Short: "Log out of a provider",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := types.Provider(args[0])

			if err := app.Auth.Logout(cmd.Context(), name); err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), terminal.Success.Render(
				fmt.Sprintf("Logged out of %s", name)))
			return nil
		},
	}
}

// newAuthStatusCommand returns the `slickcode auth status` command.
func newAuthStatusCommand(app *core.App) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the authentication state of every registered provider",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			names := app.Providers.List()
			if len(names) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), terminal.Muted.Render("No providers registered."))
				return nil
			}

			out := cmd.OutOrStdout()
			now := time.Now()
			for _, name := range names {
				fmt.Fprintf(out, "%-12s %s\n", name, sessionStatus(cmd, app, name, now))
			}
			return nil
		},
	}
}

// sessionStatus renders one provider's authentication state for
// `auth status`.
func sessionStatus(cmd *cobra.Command, app *core.App, name types.Provider, now time.Time) string {
	p, err := app.Providers.Get(name)
	if err != nil {
		return terminal.Error.Render(err.Error())
	}

	if _, ok := p.(provider.Authenticator); !ok {
		return terminal.Muted.Render("no authentication required")
	}

	sess, err := app.Auth.Session(cmd.Context(), name)
	switch {
	case errors.Is(err, auth.ErrNotFound):
		return terminal.Muted.Render("not logged in")
	case err != nil:
		return terminal.Error.Render(err.Error())
	case !sess.Valid(now):
		return terminal.Error.Render(fmt.Sprintf("session expired (%s)", sess.Method()))
	default:
		return terminal.Success.Render(fmt.Sprintf("logged in (%s)", sess.Method()))
	}
}

// chooseMethod picks the authentication method for a login: the
// --method flag when given (validated against the provider's supported
// methods), or the provider's preferred method otherwise.
func chooseMethod(supported []auth.Method, flag string) (auth.Method, error) {
	if len(supported) == 0 {
		return "", types.NewError(types.ErrorKindInternal,
			"provider advertises no authentication methods")
	}

	if flag == "" {
		return supported[0], nil
	}

	method, err := auth.ParseMethod(flag)
	if err != nil {
		return "", err
	}

	for _, m := range supported {
		if m == method {
			return method, nil
		}
	}
	return "", types.NewError(types.ErrorKindValidation,
		fmt.Sprintf("method %q is not supported by this provider (supported: %v)", method, supported))
}

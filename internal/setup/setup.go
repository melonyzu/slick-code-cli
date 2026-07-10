// Package setup implements the first-run wizard: choosing a provider,
// authenticating with it, discovering its models, picking a default,
// and saving the resulting configuration. It runs before the
// interactive UI starts and uses plain terminal prompts.
package setup

import (
	"context"
	"fmt"
	"strconv"

	"github.com/melonyzu/slick-code-cli/internal/config"
	"github.com/melonyzu/slick-code-cli/internal/core"
	"github.com/melonyzu/slick-code-cli/internal/terminal"
	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// Run walks the user through first-time configuration and returns once
// the config file is written and the chosen provider is authenticated
// and active.
func Run(ctx context.Context, app *core.App) error {
	t := app.Terminal
	t.Notify(terminal.Title.Render("Welcome to Slick Code!") + "\nLet's get you set up.\n")

	name, err := chooseProvider(t, app.Providers.List())
	if err != nil {
		return err
	}

	p, err := app.EnsureActive(ctx, name)
	if err != nil {
		return err
	}

	model, err := chooseModel(ctx, t, p.Models)
	if err != nil {
		return err
	}

	app.Config.Provider = name
	app.Config.Model = model

	path := app.Storage.ConfigFile(config.FileName)
	if err := config.Save(path, app.Config); err != nil {
		return err
	}

	t.Notify(terminal.Success.Render("\nAll set!") + " Configuration saved to " + path + "\n")
	return nil
}

// chooseProvider asks the user to pick a provider; with exactly one
// registered it is announced instead of asked.
func chooseProvider(t *terminal.Terminal, providers []types.Provider) (types.Provider, error) {
	if len(providers) == 0 {
		return "", types.NewError(types.ErrorKindInternal, "no providers are registered")
	}
	if len(providers) == 1 {
		t.Notify(fmt.Sprintf("Provider: %s (the only one available in this release)", providers[0]))
		return providers[0], nil
	}

	options := make([]string, len(providers))
	for i, p := range providers {
		options[i] = p.String()
	}
	i, err := choose(t, "Choose a provider", options)
	if err != nil {
		return "", err
	}
	return providers[i], nil
}

// chooseModel discovers the provider's models and asks the user to pick
// a default. If discovery fails, the model ID is asked for directly so
// setup still completes.
func chooseModel(ctx context.Context, t *terminal.Terminal, discover func(context.Context) ([]types.Model, error)) (string, error) {
	models, err := discover(ctx)
	if err != nil || len(models) == 0 {
		if err != nil {
			t.Notify(terminal.Error.Render("Could not list models: " + err.Error()))
		}
		return t.Prompt("Enter a model ID to use by default")
	}

	options := make([]string, len(models))
	for i, m := range models {
		options[i] = fmt.Sprintf("%s (%s)", m.Name, m.ID)
	}
	i, err := choose(t, "Choose a default model", options)
	if err != nil {
		return "", err
	}
	return models[i].ID, nil
}

// choose displays numbered options and returns the index the user
// picked; an empty answer means the first option.
func choose(t *terminal.Terminal, label string, options []string) (int, error) {
	t.Notify(label + ":")
	for i, opt := range options {
		t.Notify(fmt.Sprintf("  %2d. %s", i+1, opt))
	}

	answer, err := t.Prompt(fmt.Sprintf("Enter a number (1-%d, default 1)", len(options)))
	if err != nil {
		return 0, err
	}
	if answer == "" {
		return 0, nil
	}

	n, err := strconv.Atoi(answer)
	if err != nil || n < 1 || n > len(options) {
		return 0, types.NewError(types.ErrorKindValidation,
			fmt.Sprintf("%q is not a number between 1 and %d", answer, len(options)))
	}
	return n - 1, nil
}

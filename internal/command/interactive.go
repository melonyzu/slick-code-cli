package command

import (
	"context"
	"fmt"

	"github.com/melonyzu/slick-code-cli/internal/config"
	"github.com/melonyzu/slick-code-cli/internal/core"
	"github.com/melonyzu/slick-code-cli/internal/provider"
	"github.com/melonyzu/slick-code-cli/internal/repl"
	"github.com/melonyzu/slick-code-cli/internal/setup"
	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// runInteractive is the default `slickcode` experience: first-run setup
// when no configuration exists, session restoration or automatic
// authentication, then the interactive assistant.
func runInteractive(ctx context.Context, app *core.App) error {
	if _, err := app.Context.Refresh(ctx); err != nil {
		return fmt.Errorf("refresh project context: %w", err)
	}
	path := app.Storage.ConfigFile(config.FileName)
	if !config.Exists(path) || app.Config.Provider == "" {
		if err := setup.Run(ctx, app); err != nil {
			return err
		}
	}

	p, err := app.EnsureActive(ctx, app.Config.Provider)
	if err != nil {
		return err
	}

	modelID := app.Config.Model
	if modelID == "" {
		// Config predates model selection; fall back to the first
		// discovered model for this run without persisting the guess.
		models, merr := p.Models(ctx)
		if merr != nil {
			return merr
		}
		if len(models) == 0 {
			return types.NewError(types.ErrorKindProvider, fmt.Sprintf(
				"%s reported no available models; set one in the config file", app.Config.Provider))
		}
		modelID = models[0].ID
	}
	if validator, ok := p.(provider.ModelValidator); ok {
		if err := validator.ValidateModel(ctx, modelID); err != nil {
			return err
		}
	}

	return repl.Run(ctx, app, p, modelID)
}

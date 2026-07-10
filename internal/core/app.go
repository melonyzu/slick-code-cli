// Package core defines Slick Code's composition root: the App type that
// wires together the application's dependencies for use by
// internal/command, and the Bootstrap function that assembles one for a
// real process run.
package core

import (
	"log/slog"

	"github.com/melonyzu/slick-code-cli/internal/auth"
	"github.com/melonyzu/slick-code-cli/internal/config"
	projectcontext "github.com/melonyzu/slick-code-cli/internal/context"
	gitrepo "github.com/melonyzu/slick-code-cli/internal/git"
	"github.com/melonyzu/slick-code-cli/internal/provider"
	"github.com/melonyzu/slick-code-cli/internal/storage"
	"github.com/melonyzu/slick-code-cli/internal/terminal"
	"github.com/melonyzu/slick-code-cli/internal/tool"
)

// Params holds the dependencies required to construct an App. Using a
// struct, rather than positional constructor arguments, keeps New's call
// site readable as dependencies are added over time.
type Params struct {
	Config    *config.Config
	Logger    *slog.Logger
	Storage   *storage.Paths
	Terminal  *terminal.Terminal
	Providers *provider.Registry
	Auth      *auth.Manager
	Lifecycle *provider.Lifecycle
	Tools     *tool.Manager
	Context   *projectcontext.Service
	// Git is nil outside a Git working tree.
	Git *gitrepo.Manager
}

// App holds the dependencies shared across Slick Code's commands. It is
// constructed once during bootstrap and passed by reference to command
// constructors.
type App struct {
	Config    *config.Config
	Logger    *slog.Logger
	Storage   *storage.Paths
	Terminal  *terminal.Terminal
	Providers *provider.Registry
	Auth      *auth.Manager
	Lifecycle *provider.Lifecycle
	Tools     *tool.Manager
	Context   *projectcontext.Service
	// Git is nil outside a Git working tree.
	Git *gitrepo.Manager
}

// New returns an App built from p.
func New(p Params) *App {
	return &App{
		Config:    p.Config,
		Logger:    p.Logger,
		Storage:   p.Storage,
		Terminal:  p.Terminal,
		Providers: p.Providers,
		Auth:      p.Auth,
		Lifecycle: p.Lifecycle,
		Tools:     p.Tools,
		Context:   p.Context,
		Git:       p.Git,
	}
}

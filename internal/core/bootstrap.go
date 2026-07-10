package core

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/melonyzu/slick-code-cli/internal/auth"
	"github.com/melonyzu/slick-code-cli/internal/auth/keyring"
	"github.com/melonyzu/slick-code-cli/internal/config"
	projectcontext "github.com/melonyzu/slick-code-cli/internal/context"
	"github.com/melonyzu/slick-code-cli/internal/edit"
	gitrepo "github.com/melonyzu/slick-code-cli/internal/git"
	"github.com/melonyzu/slick-code-cli/internal/logging"
	"github.com/melonyzu/slick-code-cli/internal/provider"
	"github.com/melonyzu/slick-code-cli/internal/provider/anthropic"
	localprovider "github.com/melonyzu/slick-code-cli/internal/provider/ollama"
	openprovider "github.com/melonyzu/slick-code-cli/internal/provider/openai"
	"github.com/melonyzu/slick-code-cli/internal/storage"
	"github.com/melonyzu/slick-code-cli/internal/terminal"
	"github.com/melonyzu/slick-code-cli/internal/tool"
	"github.com/melonyzu/slick-code-cli/internal/tool/builtin"
	"github.com/melonyzu/slick-code-cli/internal/transport"
	"github.com/melonyzu/slick-code-cli/internal/workspace"
)

// Bootstrap resolves storage locations, loads and validates
// configuration, and assembles the App used for the lifetime of the
// process — including registering every built-in provider and tool.
//
// The credential store is layered: environment variables (for CI) win
// reads, the OS keyring is the durable writer, and an in-memory layer
// keeps login usable on machines without a keyring, at session-only
// durability.
func Bootstrap() (*App, error) {
	paths, err := storage.Discover()
	if err != nil {
		return nil, err
	}

	cfg, err := config.Load(paths.ConfigFile(config.FileName))
	if err != nil {
		return nil, err
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	logger, err := logging.New(cfg.LogLevel)
	if err != nil {
		return nil, fmt.Errorf("core: build logger: %w", err)
	}

	httpClient := transport.NewClient()

	registry, err := providerRegistry(httpClient, logger)
	if err != nil {
		return nil, err
	}

	store := auth.NewLayered(logger,
		auth.NewEnvStore(),
		keyring.New(),
		auth.NewMemoryStore(),
	)
	sessions := auth.NewManager(store, logger)

	workDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("core: resolve working directory: %w", err)
	}
	project, err := workspace.NewDiscoverer(logger).Discover(context.Background(), workDir)
	if err != nil {
		return nil, fmt.Errorf("core: discover workspace: %w", err)
	}
	var repositoryManager *gitrepo.Manager
	if project.IsGit {
		repository, discoverErr := gitrepo.NewDiscoverer(logger).Discover(context.Background(), workDir)
		if discoverErr != nil {
			return nil, fmt.Errorf("core: discover Git repository: %w", discoverErr)
		}
		repositoryManager, err = gitrepo.NewManager(gitrepo.ManagerParams{Repository: repository, Logger: logger})
		if err != nil {
			return nil, fmt.Errorf("core: build Git manager: %w", err)
		}
	}
	contextService, err := projectcontext.NewService(projectcontext.ServiceParams{
		Project:    project,
		Collector:  workspace.NewCollector(workspace.CollectorParams{Logger: logger}),
		Builder:    projectcontext.NewBuilder(projectcontext.DefaultTokenBudget, nil),
		CacheFile:  paths.CacheFile(contextCacheName(project.Root)),
		Logger:     logger,
		Repository: repositoryManager,
	})
	if err != nil {
		return nil, fmt.Errorf("core: build project context: %w", err)
	}
	tools, err := builtinTools(edit.NewEngine(logger), contextService, repositoryManager)
	if err != nil {
		return nil, err
	}

	return New(Params{
		Config:    cfg,
		Logger:    logger,
		Storage:   paths,
		Terminal:  terminal.New(),
		Providers: registry,
		Auth:      sessions,
		Lifecycle: provider.NewLifecycle(registry, sessions, logger),
		Tools: tool.NewManager(tools,
			tool.NewPermissionPolicy(tool.PermissionRead, tool.PermissionWrite),
			tool.ExecContext{WorkDir: project.Root},
			logger),
		Context: contextService,
		Git:     repositoryManager,
	}), nil
}

func providerRegistry(httpClient *http.Client, logger *slog.Logger) (*provider.Registry, error) {
	registry := provider.NewRegistry()
	if err := registry.Register(anthropic.New(httpClient, logger)); err != nil {
		return nil, err
	}
	openAI, err := openprovider.New(httpClient, logger)
	if err != nil {
		return nil, err
	}
	if err := registry.Register(openAI); err != nil {
		return nil, err
	}
	ollama, err := localprovider.New(httpClient, logger)
	if err != nil {
		return nil, err
	}
	if err := registry.Register(ollama); err != nil {
		return nil, err
	}
	return registry, nil
}

// builtinTools returns a registry holding every built-in tool. The
// editing tools all share one edit.Engine, so applied edits land in a
// single rollback journal.
func builtinTools(engine *edit.Engine, contextService *projectcontext.Service, repository *gitrepo.Manager) (*tool.Registry, error) {
	registry := tool.NewRegistry()
	for _, t := range []tool.Tool{
		builtin.NewReadFile(),
		builtin.NewListDirectory(),
		builtin.NewCreateFile(engine),
		builtin.NewWriteFile(engine),
		builtin.NewReplaceText(engine),
		builtin.NewInsertText(engine),
		builtin.NewDeleteText(engine),
		builtin.NewRenameFile(engine),
		builtin.NewProjectContext(contextService),
		builtin.NewGitStatus(repository),
		builtin.NewGitDiff(repository),
		builtin.NewGitCommit(repository),
		builtin.NewGitRestore(repository),
		builtin.NewGitCheckout(repository),
		builtin.NewGitBranch(repository),
	} {
		if err := registry.Register(t); err != nil {
			return nil, err
		}
	}
	return registry, nil
}

func contextCacheName(root string) string {
	name := filepath.Base(filepath.Clean(root))
	if name == "." || name == string(filepath.Separator) || name == "" {
		name = "workspace"
	}
	sum := sha256.Sum256([]byte(filepath.Clean(root)))
	return fmt.Sprintf("%s-%x-context.json", name, sum[:6])
}

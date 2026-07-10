// Package git provides provider-independent repository operations backed by
// the Git command-line interface. Providers access these operations only
// through built-in tools and the shared tool manager.
package git

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// Repository identifies one non-bare Git working tree.
type Repository struct {
	// Root is the absolute top-level working-tree directory.
	Root string

	// GitDir is the absolute repository metadata directory.
	GitDir string

	// Bare reports whether the repository has no working tree.
	Bare bool
}

// Discoverer locates Git repositories using an injected logger.
type Discoverer struct {
	executable string
	logger     *slog.Logger
}

// NewDiscoverer returns a repository discoverer using the Git executable on
// PATH.
func NewDiscoverer(logger *slog.Logger) *Discoverer {
	return &Discoverer{executable: "git", logger: gitLogger(logger)}
}

// Discover resolves the repository containing start. Bare repositories are
// rejected because Phase 9 operations require a working tree.
func (d *Discoverer) Discover(ctx context.Context, start string) (Repository, error) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return Repository{}, types.WrapError(types.ErrorKindValidation, "git: resolve starting path", err)
	}
	if info, statErr := os.Stat(abs); statErr != nil {
		return Repository{}, types.WrapError(types.ErrorKindValidation, "git: stat starting path", statErr)
	} else if !info.IsDir() {
		abs = filepath.Dir(abs)
	}

	rootValue, err := d.revParse(ctx, abs, "repository root", "--show-toplevel")
	if err != nil {
		return Repository{}, err
	}
	gitDirValue, err := d.revParse(ctx, abs, "metadata directory", "--absolute-git-dir")
	if err != nil {
		return Repository{}, err
	}
	bareValue, err := d.revParse(ctx, abs, "repository kind", "--is-bare-repository")
	if err != nil {
		return Repository{}, err
	}
	root, err := filepath.Abs(rootValue)
	if err != nil {
		return Repository{}, types.WrapError(types.ErrorKindInternal, "git: resolve repository root", err)
	}
	gitDir, err := filepath.Abs(gitDirValue)
	if err != nil {
		return Repository{}, types.WrapError(types.ErrorKindInternal, "git: resolve metadata directory", err)
	}
	repository := Repository{Root: filepath.Clean(root), GitDir: filepath.Clean(gitDir), Bare: bareValue == "true"}
	if repository.Bare {
		return Repository{}, types.NewError(types.ErrorKindUnsupportedCapability,
			"git: bare repositories are not supported")
	}
	d.logger.Info("git repository discovered", "root", repository.Root, "git_dir", repository.GitDir)
	return repository, nil
}

func (d *Discoverer) revParse(ctx context.Context, start, operation, flag string) (string, error) {
	cmd := exec.CommandContext(ctx, d.executable, "-C", start, "rev-parse", flag)
	cmd.Env = gitEnvironment()
	stdout, stderr := &cappedBuffer{limit: maxGitOutput}, &cappedBuffer{limit: maxGitOutput}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	err := cmd.Run()
	result := commandResult{
		stdout: stdout.String(), stderr: stderr.String(), err: err, code: exitCode(err),
		overflow: stdout.overflow || stderr.overflow,
	}
	if err != nil || result.overflow {
		return "", classifyCommandError(ctx, "discover "+operation, result)
	}
	return trimLineEnding(result.stdout), nil
}

func gitLogger(logger *slog.Logger) *slog.Logger {
	if logger != nil {
		return logger
	}
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

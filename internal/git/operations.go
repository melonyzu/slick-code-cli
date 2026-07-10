package git

import (
	"context"
	"strings"

	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// CommitOptions controls commit creation.
type CommitOptions struct {
	// Message is the non-empty commit message.
	Message string

	// Paths optionally commits only these tracked working-tree paths. Empty
	// commits the current index.
	Paths []string
}

// Commit records the result of a successful commit operation.
type Commit struct {
	// Hash is the abbreviated commit hash.
	Hash string `json:"hash"`
	// Summary is Git's human-readable commit summary.
	Summary string `json:"summary"`
}

// Commit creates a non-interactive commit. With no paths it commits the
// current index; with paths it commits only those tracked working-tree paths.
func (m *Manager) Commit(ctx context.Context, options CommitOptions) (Commit, error) {
	if strings.TrimSpace(options.Message) == "" {
		return Commit{}, types.NewError(types.ErrorKindValidation, "git: commit message is required")
	}
	if len(options.Message) > 10_000 {
		return Commit{}, types.NewError(types.ErrorKindValidation, "git: commit message exceeds 10000 bytes")
	}
	paths, err := m.normalizePaths(options.Paths, true)
	if err != nil {
		return Commit{}, err
	}
	args := []string{"-c", "commit.gpgSign=false", "commit", "--message", options.Message}
	if len(paths) > 0 {
		args = append(args, "--only", "--")
		args = append(args, paths...)
	}
	output, err := m.run(ctx, "create commit", args...)
	if err != nil {
		return Commit{}, err
	}
	head, err := m.head(ctx)
	if err != nil {
		return Commit{}, err
	}
	m.logger.Info("git commit created", "hash", head, "paths", len(paths))
	return Commit{Hash: head, Summary: strings.TrimSpace(output)}, nil
}

// Restore discards working-tree changes for tracked paths, restoring each path
// from the index.
func (m *Manager) Restore(ctx context.Context, paths []string) error {
	paths, err := m.normalizePaths(paths, false)
	if err != nil {
		return err
	}
	args := append([]string{"restore", "--worktree", "--"}, paths...)
	_, err = m.run(ctx, "restore working tree paths", args...)
	if err == nil {
		m.logger.Info("git paths restored", "paths", len(paths))
	}
	return err
}

// Checkout switches to an existing local branch.
func (m *Manager) Checkout(ctx context.Context, name string) (Branch, error) {
	if err := m.validateBranch(ctx, name); err != nil {
		return Branch{}, err
	}
	result := m.runRaw(ctx, "find branch", "show-ref", "--verify", "--quiet", "refs/heads/"+name)
	if result.err != nil {
		if result.code == 1 {
			return Branch{}, types.NewError(types.ErrorKindValidation, "git: local branch does not exist: "+name)
		}
		return Branch{}, classifyCommandError(ctx, "find branch", result)
	}
	if _, err := m.run(ctx, "checkout branch", "switch", name); err != nil {
		return Branch{}, err
	}
	m.logger.Info("git branch checked out", "branch", name)
	return m.CurrentBranch(ctx)
}

// CreateBranch creates and checks out a local branch from the current HEAD.
func (m *Manager) CreateBranch(ctx context.Context, name string) (Branch, error) {
	if err := m.validateBranch(ctx, name); err != nil {
		return Branch{}, err
	}
	if _, err := m.run(ctx, "create branch", "switch", "--create", name); err != nil {
		return Branch{}, err
	}
	m.logger.Info("git branch created", "branch", name)
	return m.CurrentBranch(ctx)
}

func (m *Manager) validateBranch(ctx context.Context, name string) error {
	if strings.TrimSpace(name) == "" || name != strings.TrimSpace(name) {
		return types.NewError(types.ErrorKindValidation, "git: branch name is required")
	}
	result := m.runRaw(ctx, "validate branch", "check-ref-format", "--branch", name)
	if result.err != nil {
		if ctx.Err() != nil {
			return classifyCommandError(ctx, "validate branch", result)
		}
		return types.NewError(types.ErrorKindValidation, "git: invalid branch name: "+name)
	}
	return nil
}

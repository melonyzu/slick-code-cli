package git

import (
	"context"
	"fmt"
	"strings"

	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// Branch describes the repository's current branch or detached HEAD.
type Branch struct {
	// Name is the local branch name, empty when HEAD is detached.
	Name string `json:"name,omitempty"`

	// Head is the abbreviated HEAD commit, empty in an unborn repository.
	Head string `json:"head,omitempty"`

	// Detached reports whether HEAD points directly to a commit.
	Detached bool `json:"detached"`
}

// Metadata describes repository identity and current revision state.
type Metadata struct {
	// Root is the repository working-tree root.
	Root string `json:"root"`
	// GitDir is the repository metadata directory.
	GitDir string `json:"git_dir"`
	// Bare reports whether the repository has no working tree.
	Bare bool `json:"bare"`
	// Branch is the current branch and HEAD state.
	Branch Branch `json:"branch"`
}

// Change describes one path reported by Git porcelain status.
type Change struct {
	// Path is the current repository-relative path.
	Path string `json:"path"`
	// OriginalPath is the prior path for a rename or copy.
	OriginalPath string `json:"original_path,omitempty"`
	// Index is the one-character porcelain index status.
	Index string `json:"index"`
	// Worktree is the one-character porcelain working-tree status.
	Worktree string `json:"worktree"`
	// Untracked reports whether Git has not started tracking the path.
	Untracked bool `json:"untracked"`
}

// Status describes the current branch and changed files.
type Status struct {
	// Branch is the current branch and HEAD state.
	Branch Branch `json:"branch"`
	// Clean reports whether both index and working tree are unchanged.
	Clean bool `json:"clean"`
	// Changes contains every porcelain status entry.
	Changes []Change `json:"changes"`
}

// CurrentBranch returns the current local branch and HEAD revision.
func (m *Manager) CurrentBranch(ctx context.Context) (Branch, error) {
	result := m.runRaw(ctx, "detect current branch", "symbolic-ref", "--quiet", "--short", "HEAD")
	if result.err == nil {
		head, err := m.head(ctx)
		return Branch{Name: strings.TrimSpace(result.stdout), Head: head}, err
	}
	if result.code != 1 {
		return Branch{}, classifyCommandError(ctx, "detect current branch", result)
	}
	head, err := m.head(ctx)
	if err != nil {
		return Branch{}, err
	}
	return Branch{Head: head, Detached: true}, nil
}

// Metadata returns repository root, Git directory, and current branch state.
func (m *Manager) Metadata(ctx context.Context) (Metadata, error) {
	branch, err := m.CurrentBranch(ctx)
	if err != nil {
		return Metadata{}, err
	}
	return Metadata{Root: m.repository.Root, GitDir: m.repository.GitDir, Bare: m.repository.Bare, Branch: branch}, nil
}

// Status returns porcelain working-tree and index changes.
func (m *Manager) Status(ctx context.Context) (Status, error) {
	branch, err := m.CurrentBranch(ctx)
	if err != nil {
		return Status{}, err
	}
	output, err := m.run(ctx, "read status", "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return Status{}, err
	}
	changes, err := parseStatus(output)
	if err != nil {
		return Status{}, err
	}
	return Status{Branch: branch, Clean: len(changes) == 0, Changes: changes}, nil
}

// ChangedFiles returns the status entries for every changed path.
func (m *Manager) ChangedFiles(ctx context.Context) ([]Change, error) {
	status, err := m.Status(ctx)
	if err != nil {
		return nil, err
	}
	return append([]Change(nil), status.Changes...), nil
}

func (m *Manager) head(ctx context.Context) (string, error) {
	result := m.runRaw(ctx, "resolve HEAD", "rev-parse", "--short=12", "--verify", "HEAD")
	if result.err == nil {
		return strings.TrimSpace(result.stdout), nil
	}
	if result.code == 128 && strings.Contains(strings.ToLower(result.stderr), "needed a single revision") {
		return "", nil
	}
	return "", classifyCommandError(ctx, "resolve HEAD", result)
}

func parseStatus(output string) ([]Change, error) {
	if output == "" {
		return nil, nil
	}
	records := strings.Split(output, "\x00")
	changes := make([]Change, 0, len(records))
	for i := 0; i < len(records); i++ {
		record := records[i]
		if record == "" {
			continue
		}
		if len(record) < 4 || record[2] != ' ' {
			return nil, types.NewError(types.ErrorKindInternal,
				fmt.Sprintf("git: malformed status record %q", record))
		}
		change := Change{
			Path:      record[3:],
			Index:     string(record[0]),
			Worktree:  string(record[1]),
			Untracked: record[:2] == "??",
		}
		if record[0] == 'R' || record[0] == 'C' || record[1] == 'R' || record[1] == 'C' {
			i++
			if i >= len(records) || records[i] == "" {
				return nil, types.NewError(types.ErrorKindInternal, "git: rename status is missing its original path")
			}
			change.OriginalPath = records[i]
		}
		changes = append(changes, change)
	}
	return changes, nil
}

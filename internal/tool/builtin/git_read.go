package builtin

import (
	"context"
	"encoding/json"

	gitrepo "github.com/melonyzu/slick-code-cli/internal/git"
	"github.com/melonyzu/slick-code-cli/internal/tool"
	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// GitStatus reports repository metadata and changed paths.
type GitStatus struct{ manager *gitrepo.Manager }

// NewGitStatus returns the git_status tool.
func NewGitStatus(manager *gitrepo.Manager) *GitStatus { return &GitStatus{manager: manager} }

// Definition implements tool.Tool.
func (*GitStatus) Definition() types.Tool {
	return types.Tool{
		Name:        "git_status",
		Description: "Report the current Git branch, HEAD, clean state, and changed files.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
	}
}

// Permission implements tool.Tool.
func (*GitStatus) Permission() tool.Permission { return tool.PermissionRead }

// Execute implements tool.Tool.
func (t *GitStatus) Execute(ctx context.Context, _ tool.ExecContext, input json.RawMessage) (string, error) {
	if err := requireGit(t.manager, "git_status"); err != nil {
		return "", err
	}
	var args struct{}
	if err := decodeGitInput(input, &args, "git_status"); err != nil {
		return "", err
	}
	status, err := t.manager.Status(ctx)
	if err != nil {
		return "", err
	}
	return encodeGitResult(status, "git_status")
}

// GitDiff returns working-tree or staged changes.
type GitDiff struct{ manager *gitrepo.Manager }

// NewGitDiff returns the git_diff tool.
func NewGitDiff(manager *gitrepo.Manager) *GitDiff { return &GitDiff{manager: manager} }

// Definition implements tool.Tool.
func (*GitDiff) Definition() types.Tool {
	return types.Tool{
		Name:        "git_diff",
		Description: "Return the unified Git diff for working-tree or staged changes, optionally limited to paths.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"staged":{"type":"boolean","description":"Show changes staged in the index."},
				"paths":{"type":"array","items":{"type":"string"},"description":"Optional repository-relative paths."}
			}
		}`),
	}
}

// Permission implements tool.Tool.
func (*GitDiff) Permission() tool.Permission { return tool.PermissionRead }

// Execute implements tool.Tool.
func (t *GitDiff) Execute(ctx context.Context, _ tool.ExecContext, input json.RawMessage) (string, error) {
	if err := requireGit(t.manager, "git_diff"); err != nil {
		return "", err
	}
	var args struct {
		Staged bool     `json:"staged"`
		Paths  []string `json:"paths"`
	}
	if err := decodeGitInput(input, &args, "git_diff"); err != nil {
		return "", err
	}
	diff, err := t.manager.Diff(ctx, gitrepo.DiffOptions{Staged: args.Staged, Paths: args.Paths})
	if err != nil {
		return "", err
	}
	if diff == "" {
		return "(no changes)", nil
	}
	return diff, nil
}

var (
	_ tool.Tool = (*GitStatus)(nil)
	_ tool.Tool = (*GitDiff)(nil)
)

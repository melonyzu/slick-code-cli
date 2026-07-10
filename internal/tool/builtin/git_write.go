package builtin

import (
	"context"
	"encoding/json"

	gitrepo "github.com/melonyzu/slick-code-cli/internal/git"
	"github.com/melonyzu/slick-code-cli/internal/tool"
	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// GitCommit creates a commit from the index or selected tracked paths.
type GitCommit struct{ manager *gitrepo.Manager }

// NewGitCommit returns the git_commit tool.
func NewGitCommit(manager *gitrepo.Manager) *GitCommit { return &GitCommit{manager: manager} }

// Definition implements tool.Tool.
func (*GitCommit) Definition() types.Tool {
	return types.Tool{
		Name: "git_commit",
		Description: "Create a non-interactive Git commit. With no paths, commit the staged index; " +
			"with paths, commit only those tracked working-tree paths.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"message":{"type":"string","description":"Commit message."},
				"paths":{"type":"array","items":{"type":"string"},"description":"Optional tracked paths to commit."}
			},
			"required":["message"]
		}`),
	}
}

// Permission implements tool.Tool.
func (*GitCommit) Permission() tool.Permission { return tool.PermissionWrite }

// Execute implements tool.Tool.
func (t *GitCommit) Execute(ctx context.Context, _ tool.ExecContext, input json.RawMessage) (string, error) {
	if err := requireGit(t.manager, "git_commit"); err != nil {
		return "", err
	}
	var args struct {
		Message string   `json:"message"`
		Paths   []string `json:"paths"`
	}
	if err := decodeGitInput(input, &args, "git_commit"); err != nil {
		return "", err
	}
	commit, err := t.manager.Commit(ctx, gitrepo.CommitOptions{Message: args.Message, Paths: args.Paths})
	if err != nil {
		return "", err
	}
	return encodeGitResult(commit, "git_commit")
}

// GitRestore discards working-tree changes to tracked paths.
type GitRestore struct{ manager *gitrepo.Manager }

// NewGitRestore returns the git_restore tool.
func NewGitRestore(manager *gitrepo.Manager) *GitRestore { return &GitRestore{manager: manager} }

// Definition implements tool.Tool.
func (*GitRestore) Definition() types.Tool {
	return types.Tool{
		Name:        "git_restore",
		Description: "Discard working-tree changes for tracked repository-relative paths, restoring them from the index.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{"paths":{"type":"array","items":{"type":"string"},"minItems":1}},
			"required":["paths"]
		}`),
	}
}

// Permission implements tool.Tool.
func (*GitRestore) Permission() tool.Permission { return tool.PermissionWrite }

// Execute implements tool.Tool.
func (t *GitRestore) Execute(ctx context.Context, _ tool.ExecContext, input json.RawMessage) (string, error) {
	if err := requireGit(t.manager, "git_restore"); err != nil {
		return "", err
	}
	var args struct {
		Paths []string `json:"paths"`
	}
	if err := decodeGitInput(input, &args, "git_restore"); err != nil {
		return "", err
	}
	if err := t.manager.Restore(ctx, args.Paths); err != nil {
		return "", err
	}
	return encodeGitResult(struct {
		Restored []string `json:"restored"`
	}{Restored: args.Paths}, "git_restore")
}

var (
	_ tool.Tool = (*GitCommit)(nil)
	_ tool.Tool = (*GitRestore)(nil)
)

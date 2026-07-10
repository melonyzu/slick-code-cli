package builtin

import (
	"context"
	"encoding/json"

	gitrepo "github.com/melonyzu/slick-code-cli/internal/git"
	"github.com/melonyzu/slick-code-cli/internal/tool"
	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// GitCheckout switches to an existing local branch.
type GitCheckout struct{ manager *gitrepo.Manager }

// NewGitCheckout returns the git_checkout tool.
func NewGitCheckout(manager *gitrepo.Manager) *GitCheckout { return &GitCheckout{manager: manager} }

// Definition implements tool.Tool.
func (*GitCheckout) Definition() types.Tool {
	return types.Tool{
		Name:        "git_checkout",
		Description: "Switch to an existing local Git branch without creating or merging branches.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{"branch":{"type":"string","description":"Existing local branch name."}},
			"required":["branch"]
		}`),
	}
}

// Permission implements tool.Tool.
func (*GitCheckout) Permission() tool.Permission { return tool.PermissionWrite }

// Execute implements tool.Tool.
func (t *GitCheckout) Execute(ctx context.Context, _ tool.ExecContext, input json.RawMessage) (string, error) {
	if err := requireGit(t.manager, "git_checkout"); err != nil {
		return "", err
	}
	var args struct {
		Branch string `json:"branch"`
	}
	if err := decodeGitInput(input, &args, "git_checkout"); err != nil {
		return "", err
	}
	branch, err := t.manager.Checkout(ctx, args.Branch)
	if err != nil {
		return "", err
	}
	return encodeGitResult(branch, "git_checkout")
}

// GitBranch creates and switches to a new local branch.
type GitBranch struct{ manager *gitrepo.Manager }

// NewGitBranch returns the git_branch tool.
func NewGitBranch(manager *gitrepo.Manager) *GitBranch { return &GitBranch{manager: manager} }

// Definition implements tool.Tool.
func (*GitBranch) Definition() types.Tool {
	return types.Tool{
		Name:        "git_branch",
		Description: "Create and switch to a new local Git branch from the current HEAD.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{"name":{"type":"string","description":"New local branch name."}},
			"required":["name"]
		}`),
	}
}

// Permission implements tool.Tool.
func (*GitBranch) Permission() tool.Permission { return tool.PermissionWrite }

// Execute implements tool.Tool.
func (t *GitBranch) Execute(ctx context.Context, _ tool.ExecContext, input json.RawMessage) (string, error) {
	if err := requireGit(t.manager, "git_branch"); err != nil {
		return "", err
	}
	var args struct {
		Name string `json:"name"`
	}
	if err := decodeGitInput(input, &args, "git_branch"); err != nil {
		return "", err
	}
	branch, err := t.manager.CreateBranch(ctx, args.Name)
	if err != nil {
		return "", err
	}
	return encodeGitResult(branch, "git_branch")
}

var (
	_ tool.Tool = (*GitCheckout)(nil)
	_ tool.Tool = (*GitBranch)(nil)
)

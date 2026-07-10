package builtin

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	gitrepo "github.com/melonyzu/slick-code-cli/internal/git"
	"github.com/melonyzu/slick-code-cli/internal/tool"
	"github.com/melonyzu/slick-code-cli/pkg/types"
)

func TestGitWriteToolPermissionAndDryRun(t *testing.T) {
	root, manager := gitToolRepository(t)
	writeToolFile(t, root, "tracked.txt", "changed\n")
	gitCommit := NewGitCommit(manager)
	call := tool.Request{Call: types.ToolCall{
		ID: "git-call", Name: "git_commit",
		Input: json.RawMessage(`{"message":"should not commit","paths":["tracked.txt"]}`),
	}}

	denied := gitToolManager(t, root, gitCommit, tool.NewPermissionPolicy(tool.PermissionRead)).Execute(context.Background(), call)
	if types.KindOf(denied.Err) != types.ErrorKindPermissionDenied {
		t.Fatalf("denied error = %v", denied.Err)
	}

	call.DryRun = true
	dry := gitToolManager(t, root, gitCommit, tool.NewPermissionPolicy(tool.PermissionWrite)).Execute(context.Background(), call)
	if dry.Err != nil || !strings.Contains(dry.Content, "dry run") {
		t.Fatalf("dry run = %+v", dry)
	}
	if output := runToolGit(t, root, "status", "--porcelain"); !strings.Contains(output, "tracked.txt") {
		t.Fatal("dry run changed repository state")
	}
}

func TestGitStatusTool(t *testing.T) {
	root, repository := gitToolRepository(t)
	writeToolFile(t, root, "tracked.txt", "changed\n")
	result, err := NewGitStatus(repository).Execute(context.Background(), tool.ExecContext{WorkDir: root}, nil)
	if err != nil || !strings.Contains(result, `"path":"tracked.txt"`) {
		t.Fatalf("status result=%s err=%v", result, err)
	}
}

func gitToolRepository(t *testing.T) (string, *gitrepo.Manager) {
	t.Helper()
	root := t.TempDir()
	runToolGit(t, root, "init", "--initial-branch=main")
	runToolGit(t, root, "config", "user.name", "Slick Code Tests")
	runToolGit(t, root, "config", "user.email", "slickcode@example.test")
	writeToolFile(t, root, "tracked.txt", "initial\n")
	runToolGit(t, root, "add", "tracked.txt")
	runToolGit(t, root, "commit", "-m", "initial")
	repo, err := gitrepo.NewDiscoverer(nil).Discover(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	manager, err := gitrepo.NewManager(gitrepo.ManagerParams{Repository: repo})
	if err != nil {
		t.Fatal(err)
	}
	return root, manager
}

func gitToolManager(t *testing.T, root string, gitTool tool.Tool, policy tool.Policy) *tool.Manager {
	t.Helper()
	registry := tool.NewRegistry()
	if err := registry.Register(gitTool); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return tool.NewManager(registry, policy, tool.ExecContext{WorkDir: root}, logger)
}

func runToolGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "LC_ALL=C")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return string(output)
}

func writeToolFile(t *testing.T, root, path, content string) {
	t.Helper()
	full := filepath.Join(root, path)
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

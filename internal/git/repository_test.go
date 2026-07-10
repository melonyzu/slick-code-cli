package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/melonyzu/slick-code-cli/pkg/types"
)

func TestDiscoverRepository(t *testing.T) {
	root := initRepository(t)
	nested := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	repository, err := NewDiscoverer(nil).Discover(context.Background(), nested)
	if err != nil {
		t.Fatal(err)
	}
	if repository.Root != root || repository.GitDir == "" || repository.Bare {
		t.Fatalf("repository = %+v", repository)
	}
}

func TestDiscoverRejectsNonRepository(t *testing.T) {
	_, err := NewDiscoverer(nil).Discover(context.Background(), t.TempDir())
	if types.KindOf(err) != types.ErrorKindValidation {
		t.Fatalf("error = %v, kind = %s", err, types.KindOf(err))
	}
}

func TestDiscoverPreservesNewlineInRepositoryPath(t *testing.T) {
	root := filepath.Join(t.TempDir(), "repository\nname")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "init", "--initial-branch=main")
	repository, err := NewDiscoverer(nil).Discover(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if repository.Root != root {
		t.Fatalf("root = %q, want %q", repository.Root, root)
	}
}

func initRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runGit(t, root, "init", "--initial-branch=main")
	runGit(t, root, "config", "user.name", "Slick Code Tests")
	runGit(t, root, "config", "user.email", "slickcode@example.test")
	writeGitFile(t, root, "tracked.txt", "initial\n")
	runGit(t, root, "add", "tracked.txt")
	runGit(t, root, "commit", "-m", "initial")
	return root
}

func managerFor(t *testing.T, root string) *Manager {
	t.Helper()
	repository, err := NewDiscoverer(nil).Discover(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager(ManagerParams{Repository: repository})
	if err != nil {
		t.Fatal(err)
	}
	return manager
}

func runGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	commandArgs := append([]string{"-C", root}, args...)
	cmd := exec.Command("git", commandArgs...)
	cmd.Env = gitEnvironment()
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return string(output)
}

func writeGitFile(t *testing.T, root, path, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

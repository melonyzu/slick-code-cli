package workspace

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverFindsGitRootAndNestedWorkspace(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, ".git"))
	mustWrite(t, filepath.Join(root, "go.mod"), "module example.com/root\n")
	nested := filepath.Join(root, "packages", "web")
	mustMkdir(t, nested)
	mustWrite(t, filepath.Join(nested, "package.json"), `{}`)
	start := filepath.Join(nested, "src")
	mustMkdir(t, start)

	project, err := NewDiscoverer(nil).Discover(context.Background(), start)
	if err != nil {
		t.Fatal(err)
	}
	if project.Root != root || project.GitRoot != root || !project.IsGit {
		t.Fatalf("project = %+v, want Git root %s", project, root)
	}
	if len(project.Nested) != 1 || project.Nested[0].Path != "packages/web" {
		t.Fatalf("nested workspaces = %+v", project.Nested)
	}
}

func TestDiscoverUsesNearestProjectMarkerWithoutGit(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "pyproject.toml"), "[project]\nname='demo'\n")
	start := filepath.Join(root, "src", "demo")
	mustMkdir(t, start)

	project, err := NewDiscoverer(nil).Discover(context.Background(), start)
	if err != nil {
		t.Fatal(err)
	}
	if project.Root != root || project.IsGit {
		t.Fatalf("project = %+v, want non-Git root %s", project, root)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	mustMkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

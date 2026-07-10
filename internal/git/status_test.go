package git

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestStatusAndCurrentBranch(t *testing.T) {
	root := initRepository(t)
	manager := managerFor(t, root)
	branch, err := manager.CurrentBranch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if branch.Name != "main" || branch.Head == "" || branch.Detached {
		t.Fatalf("branch = %+v", branch)
	}

	writeGitFile(t, root, "tracked.txt", "modified\n")
	writeGitFile(t, root, "untracked.txt", "new\n")
	status, err := manager.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if status.Clean || len(status.Changes) != 2 {
		t.Fatalf("status = %+v", status)
	}
	paths := []string{status.Changes[0].Path, status.Changes[1].Path}
	if !slices.Contains(paths, "tracked.txt") || !slices.Contains(paths, "untracked.txt") {
		t.Fatalf("changed paths = %v", paths)
	}
}

func TestStatusParsesRename(t *testing.T) {
	root := initRepository(t)
	runGit(t, root, "mv", "tracked.txt", "renamed.txt")
	status, err := managerFor(t, root).Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Changes) != 1 || status.Changes[0].Path != "renamed.txt" || status.Changes[0].OriginalPath != "tracked.txt" {
		t.Fatalf("status = %+v", status)
	}
}

func TestCurrentBranchInUnbornRepository(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "--initial-branch=main")
	repository, err := NewDiscoverer(nil).Discover(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager(ManagerParams{Repository: repository})
	if err != nil {
		t.Fatal(err)
	}
	branch, err := manager.CurrentBranch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if branch.Name != "main" || branch.Head != "" || branch.Detached {
		t.Fatalf("unborn branch = %+v", branch)
	}
}

func TestCurrentBranchAtDetachedHead(t *testing.T) {
	root := initRepository(t)
	runGit(t, root, "checkout", "--detach", "HEAD")
	branch, err := managerFor(t, root).CurrentBranch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if branch.Name != "" || branch.Head == "" || !branch.Detached {
		t.Fatalf("detached branch = %+v", branch)
	}
	if _, err := os.Stat(filepath.Join(root, ".git", "HEAD")); err != nil {
		t.Fatal(err)
	}
}

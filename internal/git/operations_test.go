package git

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/melonyzu/slick-code-cli/pkg/types"
)

func TestDiffWorkingAndStaged(t *testing.T) {
	root := initRepository(t)
	manager := managerFor(t, root)
	writeGitFile(t, root, "tracked.txt", "working\n")
	diff, err := manager.Diff(context.Background(), DiffOptions{})
	if err != nil || !strings.Contains(diff, "+working") {
		t.Fatalf("working diff error=%v\n%s", err, diff)
	}
	runGit(t, root, "add", "tracked.txt")
	staged, err := manager.Diff(context.Background(), DiffOptions{Staged: true})
	if err != nil || !strings.Contains(staged, "+working") {
		t.Fatalf("staged diff error=%v\n%s", err, staged)
	}
}

func TestCommitCreatesRevision(t *testing.T) {
	root := initRepository(t)
	manager := managerFor(t, root)
	writeGitFile(t, root, "tracked.txt", "committed\n")
	commit, err := manager.Commit(context.Background(), CommitOptions{
		Message: "update tracked file", Paths: []string{"tracked.txt"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if commit.Hash == "" || !strings.Contains(runGit(t, root, "log", "-1", "--pretty=%s"), "update tracked file") {
		t.Fatalf("commit = %+v", commit)
	}
	status, err := manager.Status(context.Background())
	if err != nil || !status.Clean {
		t.Fatalf("status after commit = %+v err=%v", status, err)
	}
}

func TestRestoreTrackedFile(t *testing.T) {
	root := initRepository(t)
	manager := managerFor(t, root)
	writeGitFile(t, root, "tracked.txt", "discard me\n")
	if err := manager.Restore(context.Background(), []string{"tracked.txt"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "tracked.txt"))
	if err != nil || string(data) != "initial\n" {
		t.Fatalf("restored content = %q err=%v", data, err)
	}
}

func TestBranchCreateAndCheckout(t *testing.T) {
	root := initRepository(t)
	manager := managerFor(t, root)
	created, err := manager.CreateBranch(context.Background(), "feature/test")
	if err != nil || created.Name != "feature/test" {
		t.Fatalf("create branch = %+v err=%v", created, err)
	}
	checkedOut, err := manager.Checkout(context.Background(), "main")
	if err != nil || checkedOut.Name != "main" {
		t.Fatalf("checkout = %+v err=%v", checkedOut, err)
	}
}

func TestInvalidOperationsReturnStructuredErrors(t *testing.T) {
	manager := managerFor(t, initRepository(t))
	if _, err := manager.Checkout(context.Background(), "missing"); types.KindOf(err) != types.ErrorKindValidation {
		t.Fatalf("checkout error = %v kind=%s", err, types.KindOf(err))
	}
	if err := manager.Restore(context.Background(), []string{"../outside"}); types.KindOf(err) != types.ErrorKindPermissionDenied {
		t.Fatalf("restore error = %v kind=%s", err, types.KindOf(err))
	}
	if _, err := manager.Commit(context.Background(), CommitOptions{Message: "nothing"}); types.KindOf(err) != types.ErrorKindConflict {
		t.Fatalf("empty commit error = %v kind=%s", err, types.KindOf(err))
	}
}

func TestCanceledAndTimedOutOperationsAreStructured(t *testing.T) {
	manager := managerFor(t, initRepository(t))
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := manager.Status(canceled); types.KindOf(err) != types.ErrorKindCanceled {
		t.Fatalf("canceled error = %v kind=%s", err, types.KindOf(err))
	}
	timedOut, cancelTimeout := context.WithTimeout(context.Background(), 0)
	defer cancelTimeout()
	if _, err := manager.Diff(timedOut, DiffOptions{}); types.KindOf(err) != types.ErrorKindTimeout {
		t.Fatalf("timeout error = %v kind=%s", err, types.KindOf(err))
	}
}

func TestCommitDoesNotExecuteRepositoryHooks(t *testing.T) {
	root := initRepository(t)
	hook := filepath.Join(root, ".git", "hooks", "pre-commit")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeGitFile(t, root, "tracked.txt", "safe\n")
	commit, err := managerFor(t, root).Commit(context.Background(), CommitOptions{
		Message: "hook isolated", Paths: []string{"tracked.txt"},
	})
	if err != nil || commit.Hash == "" {
		t.Fatalf("commit = %+v error = %v", commit, err)
	}
}

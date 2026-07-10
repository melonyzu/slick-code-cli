package projectcontext

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/melonyzu/slick-code-cli/internal/workspace"
)

func TestServicePersistsCacheAndReportsIncrementalChanges(t *testing.T) {
	root := t.TempDir()
	writeContextFile(t, filepath.Join(root, "main.go"), "package main\n")
	cacheFile := filepath.Join(t.TempDir(), "context.json")
	params := ServiceParams{
		Project:   workspace.Project{Root: root},
		Collector: workspace.NewCollector(workspace.CollectorParams{}),
		Builder:   NewBuilder(10_000, nil),
		CacheFile: cacheFile,
	}
	service, err := NewService(params)
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.Refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Changed) != 1 || service.Snapshot().IncludedFiles != 1 {
		t.Fatalf("first refresh = %+v snapshot=%+v", first, service.Snapshot())
	}
	if info, err := os.Stat(cacheFile); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("cache mode: info=%v err=%v", info, err)
	}

	reloaded, err := NewService(params)
	if err != nil {
		t.Fatal(err)
	}
	second, err := reloaded.Refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if second.Reused != 1 || len(second.Changed) != 0 {
		t.Fatalf("cached refresh = %+v", second)
	}

	if err := os.Remove(filepath.Join(root, "main.go")); err != nil {
		t.Fatal(err)
	}
	third, err := reloaded.Refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(third.Removed) != 1 || third.Removed[0] != "main.go" {
		t.Fatalf("removed refresh = %+v", third)
	}
}

func writeContextFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

package workspace

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestParseIgnoreAndMatcher(t *testing.T) {
	patterns, err := ParseIgnore(strings.NewReader("# comment\n*.log\n!important.log\n/build/\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(patterns) != 3 || !patterns[1].Negated || !patterns[2].DirectoryOnly || !patterns[2].Anchored {
		t.Fatalf("patterns = %+v", patterns)
	}

	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ".gitignore"), "*.log\n!important.log\n/build/\n")
	matcher := NewIgnoreMatcher(root)
	if err := matcher.AddFile(filepath.Join(root, ".gitignore")); err != nil {
		t.Fatal(err)
	}
	if !matcher.Ignored("debug.log", false) || matcher.Ignored("important.log", false) {
		t.Fatal("negation did not restore important.log")
	}
	if !matcher.Ignored("build/output.txt", false) || matcher.Ignored("src/build.go", false) {
		t.Fatal("anchored build directory pattern matched incorrectly")
	}
}

func TestCollectorIgnoresBinaryAndRefreshesIncrementally(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ".gitignore"), "ignored.txt\n")
	mustWrite(t, filepath.Join(root, "main.go"), "package main\n")
	mustWrite(t, filepath.Join(root, "ignored.txt"), "secret\n")
	outside := filepath.Join(t.TempDir(), "outside.txt")
	mustWrite(t, outside, "outside\n")
	if err := os.Symlink(outside, filepath.Join(root, "linked.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "image.bin"), []byte{0, 1, 2}, 0o644); err != nil {
		t.Fatal(err)
	}
	project := Project{Root: root}
	collector := NewCollector(CollectorParams{})

	first, err := collector.Collect(context.Background(), project, nil)
	if err != nil {
		t.Fatal(err)
	}
	if paths(first.Files); !slices.Equal(paths(first.Files), []string{".gitignore", "main.go"}) {
		t.Fatalf("collected paths = %v", paths(first.Files))
	}
	if DetectLanguage("main.go", "") != "Go" {
		t.Fatal("Go language was not detected")
	}

	second, err := collector.Collect(context.Background(), project, first.Files)
	if err != nil {
		t.Fatal(err)
	}
	if second.Reused != len(first.Files) || len(second.Changed) != 0 {
		t.Fatalf("unchanged refresh = %+v", second)
	}

	time.Sleep(2 * time.Millisecond)
	mustWrite(t, filepath.Join(root, "main.go"), "package main\n\nfunc main() {}\n")
	third, err := collector.Collect(context.Background(), project, second.Files)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(third.Changed, []string{"main.go"}) || third.Reused != 1 {
		t.Fatalf("incremental refresh = %+v", third)
	}
}

func paths(files []File) []string {
	result := make([]string, 0, len(files))
	for _, file := range files {
		result = append(result, file.Path)
	}
	return result
}

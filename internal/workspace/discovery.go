// Package workspace discovers project boundaries and collects source files
// without depending on a particular language, provider, or version-control
// command.
package workspace

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Project describes the workspace containing the process's starting path.
type Project struct {
	StartDir string            `json:"start_dir"`
	Root     string            `json:"root"`
	GitRoot  string            `json:"git_root,omitempty"`
	IsGit    bool              `json:"is_git"`
	Nested   []NestedWorkspace `json:"nested,omitempty"`
}

// NestedWorkspace is a project boundary below the selected root.
type NestedWorkspace struct {
	Path    string   `json:"path"`
	Markers []string `json:"markers"`
}

// Discoverer resolves workspace boundaries.
type Discoverer struct {
	logger *slog.Logger
}

// NewDiscoverer returns a workspace discoverer using logger for diagnostics.
func NewDiscoverer(logger *slog.Logger) *Discoverer {
	return &Discoverer{logger: loggerOrDiscard(logger)}
}

// Discover finds the Git repository or nearest project marker containing
// start. A containing Git repository is the workspace root; without one, the
// nearest directory containing a recognized project marker is used.
func (d *Discoverer) Discover(ctx context.Context, start string) (Project, error) {
	startDir, err := resolveStart(start)
	if err != nil {
		return Project{}, err
	}

	var gitRoot, markerRoot string
	for dir := startDir; ; dir = filepath.Dir(dir) {
		if err := ctx.Err(); err != nil {
			return Project{}, err
		}
		if gitRoot == "" && hasMarker(dir, ".git") {
			gitRoot = dir
		}
		if markerRoot == "" && len(markersAt(dir, false)) > 0 {
			markerRoot = dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}

	root := gitRoot
	if root == "" {
		root = markerRoot
	}
	if root == "" {
		root = startDir
	}

	nested, err := discoverNested(ctx, root)
	if err != nil {
		return Project{}, err
	}
	project := Project{
		StartDir: startDir,
		Root:     root,
		GitRoot:  gitRoot,
		IsGit:    gitRoot != "",
		Nested:   nested,
	}
	d.logger.Info("workspace discovered",
		"root", project.Root, "git", project.IsGit, "nested", len(project.Nested))
	return project, nil
}

func resolveStart(start string) (string, error) {
	if start == "" {
		return "", fmt.Errorf("workspace: starting path is empty")
	}
	abs, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("workspace: resolve %s: %w", start, err)
	}
	if resolved, evalErr := filepath.EvalSymlinks(abs); evalErr == nil {
		abs = resolved
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("workspace: stat %s: %w", abs, err)
	}
	if !info.IsDir() {
		abs = filepath.Dir(abs)
	}
	return filepath.Clean(abs), nil
}

const nestedDiscoveryDepth = 12

func discoverNested(ctx context.Context, root string) ([]NestedWorkspace, error) {
	var nested []NestedWorkspace
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		if path != root && hardIgnoredDirectory(entry.Name()) {
			return filepath.SkipDir
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel != "." && strings.Count(filepath.ToSlash(rel), "/")+1 > nestedDiscoveryDepth {
			return filepath.SkipDir
		}
		if path == root {
			return nil
		}
		markers := markersAt(path, true)
		if len(markers) > 0 {
			nested = append(nested, NestedWorkspace{
				Path:    filepath.ToSlash(rel),
				Markers: markers,
			})
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("workspace: discover nested projects: %w", err)
	}
	sort.Slice(nested, func(i, j int) bool { return nested[i].Path < nested[j].Path })
	return nested, nil
}

var projectMarkers = []string{
	"go.work", "go.mod", "package.json", "pnpm-workspace.yaml",
	"pyproject.toml", "requirements.txt", "Cargo.toml", "Gemfile",
	"composer.json", "pom.xml", "build.gradle", "build.gradle.kts",
	"Package.swift", "CMakeLists.txt", "Makefile",
}

func markersAt(dir string, includeGit bool) []string {
	markers := make([]string, 0, 2)
	if includeGit && hasMarker(dir, ".git") {
		markers = append(markers, ".git")
	}
	for _, marker := range projectMarkers {
		if hasMarker(dir, marker) {
			markers = append(markers, marker)
		}
	}
	return markers
}

func hasMarker(dir, name string) bool {
	_, err := os.Lstat(filepath.Join(dir, name))
	return err == nil
}

func loggerOrDiscard(logger *slog.Logger) *slog.Logger {
	if logger != nil {
		return logger
	}
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

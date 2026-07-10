package git

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// DiffOptions controls a working-tree or staged diff.
type DiffOptions struct {
	// Staged selects changes already added to the index.
	Staged bool

	// Paths optionally limits the diff to repository-relative paths.
	Paths []string
}

// Diff returns a bounded unified diff for the selected paths.
func (m *Manager) Diff(ctx context.Context, options DiffOptions) (string, error) {
	paths, err := m.normalizePaths(options.Paths, true)
	if err != nil {
		return "", err
	}
	args := []string{"diff", "--no-ext-diff", "--no-textconv"}
	if options.Staged {
		args = append(args, "--cached")
	}
	args = append(args, "--")
	args = append(args, paths...)
	operation := "read working tree diff"
	if options.Staged {
		operation = "read staged diff"
	}
	return m.run(ctx, operation, args...)
}

func (m *Manager) normalizePaths(paths []string, allowEmpty bool) ([]string, error) {
	if len(paths) == 0 && !allowEmpty {
		return nil, types.NewError(types.ErrorKindValidation, "git: at least one path is required")
	}
	normalized := make([]string, 0, len(paths))
	for _, path := range paths {
		if path == "" || strings.HasPrefix(path, ":") {
			return nil, types.NewError(types.ErrorKindValidation, fmt.Sprintf("git: invalid path %q", path))
		}
		if filepath.IsAbs(path) {
			rel, err := filepath.Rel(m.repository.Root, filepath.Clean(path))
			if err != nil {
				return nil, types.WrapError(types.ErrorKindValidation, "git: resolve path", err)
			}
			path = rel
		}
		path = filepath.Clean(path)
		if path == "." || path == ".." || strings.HasPrefix(path, ".."+string(filepath.Separator)) {
			return nil, types.NewError(types.ErrorKindPermissionDenied,
				fmt.Sprintf("git: path %q is outside the repository", path))
		}
		normalized = append(normalized, filepath.ToSlash(path))
	}
	return normalized, nil
}

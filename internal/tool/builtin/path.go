// Package builtin implements Slick Code's built-in tools. Each tool is
// its own type implementing tool.Tool; all of them resolve paths inside
// the execution context's working directory and refuse to escape it.
package builtin

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/melonyzu/slick-code-cli/internal/tool"
	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// resolve turns a tool-supplied path into an absolute path inside
// exec.WorkDir. Relative paths resolve against the working directory;
// any path — relative or absolute — that lands outside it is refused
// with a permission error, so a tool call can never reach files the
// user didn't put in scope.
func resolve(exec tool.ExecContext, path string) (string, error) {
	if path == "" {
		path = "."
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(exec.WorkDir, path)
	}
	path = filepath.Clean(path)

	rel, err := filepath.Rel(exec.WorkDir, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", types.NewError(types.ErrorKindPermissionDenied,
			fmt.Sprintf("path %q is outside the working directory", path))
	}
	return path, nil
}

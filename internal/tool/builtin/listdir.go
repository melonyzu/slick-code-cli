package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/melonyzu/slick-code-cli/internal/tool"
	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// ListDirectory is the built-in tool that lists a directory's entries.
type ListDirectory struct{}

// NewListDirectory returns the list_directory tool.
func NewListDirectory() *ListDirectory {
	return &ListDirectory{}
}

// Definition implements tool.Tool.
func (*ListDirectory) Definition() types.Tool {
	return types.Tool{
		Name: "list_directory",
		Description: "List the entries of a directory, one per line; " +
			"directories are marked with a trailing slash. " +
			"The path is relative to the working directory and defaults to it.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "Path of the directory to list, relative to the working directory. Defaults to the working directory itself."
				}
			}
		}`),
	}
}

// Permission implements tool.Tool.
func (*ListDirectory) Permission() tool.Permission {
	return tool.PermissionRead
}

// Execute implements tool.Tool.
func (*ListDirectory) Execute(ctx context.Context, exec tool.ExecContext, input json.RawMessage) (string, error) {
	var args struct {
		Path string `json:"path"`
	}
	if len(input) > 0 {
		if err := json.Unmarshal(input, &args); err != nil {
			return "", types.WrapError(types.ErrorKindValidation, "list_directory: invalid input", err)
		}
	}

	path, err := resolve(exec, args.Path)
	if err != nil {
		return "", err
	}

	entries, err := os.ReadDir(path)
	switch {
	case os.IsNotExist(err):
		return "", types.NewError(types.ErrorKindValidation,
			fmt.Sprintf("list_directory: %s does not exist", displayPath(args.Path)))
	case err != nil:
		return "", types.WrapError(types.ErrorKindInternal,
			"list_directory: read "+displayPath(args.Path), err)
	}

	if len(entries) == 0 {
		return "(empty directory)", nil
	}

	var b strings.Builder
	for _, e := range entries {
		b.WriteString(e.Name())
		if e.IsDir() {
			b.WriteString("/")
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

// displayPath names a possibly-empty path argument in error messages.
func displayPath(path string) string {
	if path == "" {
		return "the working directory"
	}
	return path
}

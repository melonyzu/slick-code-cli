package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/melonyzu/slick-code-cli/internal/tool"
	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// maxReadBytes caps how much of a file the read_file tool returns, so a
// huge file can't blow up a model request.
const maxReadBytes = 256 * 1024

// ReadFile is the built-in tool that returns a file's contents.
type ReadFile struct{}

// NewReadFile returns the read_file tool.
func NewReadFile() *ReadFile {
	return &ReadFile{}
}

// Definition implements tool.Tool.
func (*ReadFile) Definition() types.Tool {
	return types.Tool{
		Name: "read_file",
		Description: "Read a text file and return its contents. " +
			"The path is relative to the working directory.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "Path of the file to read, relative to the working directory."
				}
			},
			"required": ["path"]
		}`),
	}
}

// Permission implements tool.Tool.
func (*ReadFile) Permission() tool.Permission {
	return tool.PermissionRead
}

// Execute implements tool.Tool.
func (*ReadFile) Execute(ctx context.Context, exec tool.ExecContext, input json.RawMessage) (string, error) {
	var args struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return "", types.WrapError(types.ErrorKindValidation, "read_file: invalid input", err)
	}
	if args.Path == "" {
		return "", types.NewError(types.ErrorKindValidation, "read_file: path is required")
	}

	path, err := resolve(exec, args.Path)
	if err != nil {
		return "", err
	}

	info, err := os.Stat(path)
	switch {
	case os.IsNotExist(err):
		return "", types.NewError(types.ErrorKindValidation,
			fmt.Sprintf("read_file: %s does not exist", args.Path))
	case err != nil:
		return "", types.WrapError(types.ErrorKindInternal, "read_file: stat "+args.Path, err)
	case info.IsDir():
		return "", types.NewError(types.ErrorKindValidation,
			fmt.Sprintf("read_file: %s is a directory — use list_directory", args.Path))
	case info.Size() > maxReadBytes:
		return "", types.NewError(types.ErrorKindValidation, fmt.Sprintf(
			"read_file: %s is %d bytes, over the %d byte limit", args.Path, info.Size(), maxReadBytes))
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", types.WrapError(types.ErrorKindInternal, "read_file: read "+args.Path, err)
	}
	return string(data), nil
}

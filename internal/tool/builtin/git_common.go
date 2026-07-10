package builtin

import (
	"encoding/json"

	gitrepo "github.com/melonyzu/slick-code-cli/internal/git"
	"github.com/melonyzu/slick-code-cli/pkg/types"
)

func requireGit(manager *gitrepo.Manager, toolName string) error {
	if manager != nil {
		return nil
	}
	return types.NewError(types.ErrorKindUnsupportedCapability,
		toolName+": the current workspace is not a Git repository")
}

func decodeGitInput(input json.RawMessage, target any, toolName string) error {
	if len(input) == 0 {
		input = json.RawMessage(`{}`)
	}
	if err := json.Unmarshal(input, target); err != nil {
		return types.WrapError(types.ErrorKindValidation, toolName+": invalid input", err)
	}
	return nil
}

func encodeGitResult(value any, toolName string) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", types.WrapError(types.ErrorKindInternal, toolName+": encode result", err)
	}
	return string(data), nil
}

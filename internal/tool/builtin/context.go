package builtin

import (
	"context"
	"encoding/json"

	projectcontext "github.com/melonyzu/slick-code-cli/internal/context"
	"github.com/melonyzu/slick-code-cli/internal/tool"
	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// ProjectContext exposes project-context status and incremental refresh to the
// assistant through the existing tool execution policy.
type ProjectContext struct {
	service *projectcontext.Service
}

// NewProjectContext returns the project_context tool.
func NewProjectContext(service *projectcontext.Service) *ProjectContext {
	return &ProjectContext{service: service}
}

// Definition implements tool.Tool.
func (*ProjectContext) Definition() types.Tool {
	return types.Tool{
		Name: "project_context",
		Description: "Inspect the automatically collected project context or incrementally refresh it " +
			"after files change. The result reports coverage and token usage, not file contents.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"refresh": {
					"type": "boolean",
					"description": "Refresh changed and removed project files before reporting status."
				}
			}
		}`),
	}
}

// Permission implements tool.Tool.
func (*ProjectContext) Permission() tool.Permission {
	return tool.PermissionRead
}

// Execute implements tool.Tool.
func (t *ProjectContext) Execute(ctx context.Context, _ tool.ExecContext, input json.RawMessage) (string, error) {
	if t.service == nil {
		return "", types.NewError(types.ErrorKindInternal, "project_context: service is unavailable")
	}
	var args struct {
		Refresh bool `json:"refresh"`
	}
	if len(input) > 0 {
		if err := json.Unmarshal(input, &args); err != nil {
			return "", types.WrapError(types.ErrorKindValidation, "project_context: invalid input", err)
		}
	}

	var changed, removed int
	if args.Refresh {
		result, err := t.service.Refresh(ctx)
		if err != nil {
			return "", types.WrapError(types.ErrorKindInternal, "project_context: refresh", err)
		}
		changed, removed = len(result.Changed), len(result.Removed)
	}
	snapshot := t.service.Snapshot()
	status := struct {
		Root          string `json:"root"`
		TotalFiles    int    `json:"total_files"`
		IncludedFiles int    `json:"included_files"`
		Estimated     int    `json:"estimated_tokens"`
		Budget        int    `json:"token_budget"`
		Truncated     bool   `json:"truncated"`
		Changed       int    `json:"changed,omitempty"`
		Removed       int    `json:"removed,omitempty"`
	}{
		Root: snapshot.Root, TotalFiles: snapshot.TotalFiles,
		IncludedFiles: snapshot.IncludedFiles, Estimated: snapshot.Estimated,
		Budget: snapshot.Budget, Truncated: snapshot.Truncated,
		Changed: changed, Removed: removed,
	}
	data, err := json.Marshal(status)
	if err != nil {
		return "", types.WrapError(types.ErrorKindInternal, "project_context: encode status", err)
	}
	return string(data), nil
}

var _ tool.Tool = (*ProjectContext)(nil)

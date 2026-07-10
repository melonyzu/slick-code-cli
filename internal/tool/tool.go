// Package tool defines the framework through which the assistant
// executes tools on the user's machine: the Tool interface implemented
// by every tool, a Registry the application discovers tools from, and a
// Manager that runs tool calls under permission checks, a time budget,
// and structured error reporting. Built-in tools live in the builtin
// subpackage; this package knows nothing about any specific tool.
package tool

import (
	"context"
	"encoding/json"
	"time"

	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// DefaultTimeout bounds a single tool execution when the ExecContext
// does not set its own budget.
const DefaultTimeout = 30 * time.Second

// Permission is the level of access a tool requires to execute.
// Permissions are checked by the Manager's Policy before every call.
type Permission string

// Access levels, from least to most invasive.
const (
	// PermissionRead covers tools that only inspect state: reading
	// files, listing directories.
	PermissionRead Permission = "read"

	// PermissionWrite covers tools that modify state: writing, moving,
	// or deleting files.
	PermissionWrite Permission = "write"

	// PermissionExecute covers tools that run external programs.
	PermissionExecute Permission = "execute"
)

// ExecContext is the environment tool executions run in. It is fixed at
// construction of the Manager and shared by every call it executes.
type ExecContext struct {
	// WorkDir is the directory relative paths resolve against. Tools
	// must refuse paths that escape it.
	WorkDir string

	// Timeout bounds a single tool execution. Zero means
	// DefaultTimeout.
	Timeout time.Duration
}

// Tool is one executable capability offered to the model. Implementations
// must be safe for concurrent use.
type Tool interface {
	// Definition describes the tool to the model: its name, what it
	// does, and the JSON Schema of its arguments.
	Definition() types.Tool

	// Permission is the access level executing this tool requires.
	Permission() Permission

	// Execute runs the tool with input, JSON matching the definition's
	// input schema, and returns the content presented to the model.
	// Failures are reported as classified types.Error values.
	Execute(ctx context.Context, exec ExecContext, input json.RawMessage) (string, error)
}

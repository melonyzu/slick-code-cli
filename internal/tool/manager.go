package tool

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// Request is one tool invocation to execute.
type Request struct {
	// Call is the model's tool call: its correlation ID, the tool name,
	// and the JSON input.
	Call types.ToolCall

	// DryRun resolves the tool and checks permissions without
	// executing, reporting what would run instead.
	DryRun bool
}

// Result is the outcome of executing a Request. Failures are carried in
// Err as classified types.Error values rather than returned separately,
// so a Result can always be handed back to the model.
type Result struct {
	// ToolCallID identifies the ToolCall this result answers.
	ToolCallID string

	// Content is the tool's output when execution succeeded.
	Content string

	// Err is the classified failure, nil on success.
	Err error
}

// ToolResult converts the Result into the domain type returned to the
// model.
func (r Result) ToolResult() types.ToolResult {
	if r.Err != nil {
		return types.ToolResult{ToolCallID: r.ToolCallID, Content: r.Err.Error(), IsError: true}
	}
	return types.ToolResult{ToolCallID: r.ToolCallID, Content: r.Content}
}

// Manager executes tool calls: it resolves the tool from the registry,
// checks the permission policy, enforces the execution time budget, and
// classifies every failure into a types.Error.
type Manager struct {
	registry *Registry
	policy   Policy
	exec     ExecContext
	logger   *slog.Logger
}

// NewManager returns a Manager executing tools from registry under
// policy, inside the environment described by exec.
func NewManager(registry *Registry, policy Policy, exec ExecContext, logger *slog.Logger) *Manager {
	if exec.Timeout <= 0 {
		exec.Timeout = DefaultTimeout
	}
	return &Manager{registry: registry, policy: policy, exec: exec, logger: logger}
}

// Registry returns the registry the manager executes tools from, for
// tool discovery (listing tools, collecting request definitions).
func (m *Manager) Registry() *Registry {
	return m.registry
}

// Execute runs one tool call. It never returns a Go error; every
// failure — unknown tool, denied permission, bad input, timeout — is
// folded into the Result so callers can hand it straight to the model.
func (m *Manager) Execute(ctx context.Context, req Request) Result {
	t, err := m.registry.Get(req.Call.Name)
	if err != nil {
		return m.fail(req, err)
	}

	if err := m.policy.Allow(req.Call.Name, t.Permission()); err != nil {
		return m.fail(req, err)
	}

	if req.DryRun {
		return Result{ToolCallID: req.Call.ID, Content: fmt.Sprintf(
			"dry run: %s would execute with input %s", req.Call.Name, req.Call.Input)}
	}

	ctx, cancel := context.WithTimeout(ctx, m.exec.Timeout)
	defer cancel()

	content, err := t.Execute(ctx, m.exec, req.Call.Input)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			err = types.WrapError(types.ErrorKindTimeout, fmt.Sprintf(
				"tool %s exceeded its %s time budget", req.Call.Name, m.exec.Timeout), err)
		}
		return m.fail(req, err)
	}

	m.logger.Debug("tool executed", "tool", req.Call.Name, "call_id", req.Call.ID)
	return Result{ToolCallID: req.Call.ID, Content: content}
}

// fail logs and wraps a failed call into a Result.
func (m *Manager) fail(req Request, err error) Result {
	m.logger.Warn("tool call failed",
		"tool", req.Call.Name, "call_id", req.Call.ID,
		"kind", types.KindOf(err), "error", err)
	return Result{ToolCallID: req.Call.ID, Err: err}
}

package tool

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/melonyzu/slick-code-cli/pkg/types"
)

func newTestManager(t *testing.T, tools []Tool, policy Policy, exec ExecContext) *Manager {
	t.Helper()
	r := NewRegistry()
	for _, tl := range tools {
		if err := r.Register(tl); err != nil {
			t.Fatalf("Register: %v", err)
		}
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewManager(r, policy, exec, logger)
}

func call(name, input string) Request {
	return Request{Call: types.ToolCall{
		ID:    "call_1",
		Name:  name,
		Input: json.RawMessage(input),
	}}
}

func TestManagerExecutesAllowedTool(t *testing.T) {
	m := newTestManager(t,
		[]Tool{&fakeTool{name: "fake", perm: PermissionRead}},
		NewPermissionPolicy(PermissionRead), ExecContext{})

	res := m.Execute(context.Background(), call("fake", `{}`))
	if res.Err != nil {
		t.Fatalf("Execute failed: %v", res.Err)
	}
	if res.Content != "ok" || res.ToolCallID != "call_1" {
		t.Errorf("Result = %+v, want content %q for call_1", res, "ok")
	}

	tr := res.ToolResult()
	if tr.IsError || tr.Content != "ok" || tr.ToolCallID != "call_1" {
		t.Errorf("ToolResult() = %+v, want successful result for call_1", tr)
	}
}

func TestManagerDeniesUngrantedPermission(t *testing.T) {
	executed := false
	m := newTestManager(t,
		[]Tool{&fakeTool{name: "writer", perm: PermissionWrite,
			execute: func(context.Context, ExecContext, json.RawMessage) (string, error) {
				executed = true
				return "", nil
			}}},
		NewPermissionPolicy(PermissionRead), ExecContext{})

	res := m.Execute(context.Background(), call("writer", `{}`))
	if executed {
		t.Error("tool executed despite the permission denial")
	}
	if kind := types.KindOf(res.Err); kind != types.ErrorKindPermissionDenied {
		t.Errorf("error kind = %q, want %q", kind, types.ErrorKindPermissionDenied)
	}
	if tr := res.ToolResult(); !tr.IsError {
		t.Error("ToolResult() of a denied call must set IsError")
	}
}

func TestManagerReportsUnknownTool(t *testing.T) {
	m := newTestManager(t, nil, NewPermissionPolicy(PermissionRead), ExecContext{})

	res := m.Execute(context.Background(), call("missing", `{}`))
	if kind := types.KindOf(res.Err); kind != types.ErrorKindValidation {
		t.Errorf("error kind = %q, want %q", kind, types.ErrorKindValidation)
	}
}

func TestManagerDryRunSkipsExecution(t *testing.T) {
	executed := false
	m := newTestManager(t,
		[]Tool{&fakeTool{name: "fake", perm: PermissionRead,
			execute: func(context.Context, ExecContext, json.RawMessage) (string, error) {
				executed = true
				return "", nil
			}}},
		NewPermissionPolicy(PermissionRead), ExecContext{})

	req := call("fake", `{"path":"x"}`)
	req.DryRun = true
	res := m.Execute(context.Background(), req)

	if executed {
		t.Error("dry run executed the tool")
	}
	if res.Err != nil {
		t.Fatalf("dry run failed: %v", res.Err)
	}
	if !strings.Contains(res.Content, "dry run") || !strings.Contains(res.Content, "fake") {
		t.Errorf("dry run content %q does not describe the skipped call", res.Content)
	}
}

func TestManagerDryRunStillChecksPermissions(t *testing.T) {
	m := newTestManager(t,
		[]Tool{&fakeTool{name: "writer", perm: PermissionWrite}},
		NewPermissionPolicy(PermissionRead), ExecContext{})

	req := call("writer", `{}`)
	req.DryRun = true
	res := m.Execute(context.Background(), req)
	if kind := types.KindOf(res.Err); kind != types.ErrorKindPermissionDenied {
		t.Errorf("dry run error kind = %q, want %q", kind, types.ErrorKindPermissionDenied)
	}
}

func TestManagerEnforcesTimeout(t *testing.T) {
	slow := &fakeTool{name: "slow", perm: PermissionRead,
		execute: func(ctx context.Context, _ ExecContext, _ json.RawMessage) (string, error) {
			<-ctx.Done()
			return "", ctx.Err()
		}}
	m := newTestManager(t, []Tool{slow},
		NewPermissionPolicy(PermissionRead),
		ExecContext{Timeout: 10 * time.Millisecond})

	res := m.Execute(context.Background(), call("slow", `{}`))
	if kind := types.KindOf(res.Err); kind != types.ErrorKindTimeout {
		t.Fatalf("error kind = %q (%v), want %q", kind, res.Err, types.ErrorKindTimeout)
	}
	if !errors.Is(res.Err, context.DeadlineExceeded) {
		t.Error("timeout error must wrap context.DeadlineExceeded")
	}
}

package builtin

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/melonyzu/slick-code-cli/internal/edit"
	"github.com/melonyzu/slick-code-cli/internal/tool"
	"github.com/melonyzu/slick-code-cli/pkg/types"
)

func testEngine() *edit.Engine {
	return edit.NewEngine(slog.New(slog.NewTextHandler(io.Discard, nil)))
}

// editManager wires the editing tools into a real tool.Manager, the
// same execution path providers use, under the given permission policy.
func editManager(t *testing.T, exec tool.ExecContext, policy tool.Policy) *tool.Manager {
	t.Helper()
	engine := testEngine()
	registry := tool.NewRegistry()
	for _, tl := range []tool.Tool{
		NewCreateFile(engine),
		NewWriteFile(engine),
		NewReplaceText(engine),
		NewInsertText(engine),
		NewDeleteText(engine),
		NewRenameFile(engine),
	} {
		if err := registry.Register(tl); err != nil {
			t.Fatalf("Register: %v", err)
		}
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return tool.NewManager(registry, policy, exec, logger)
}

func editCall(name, input string) tool.Request {
	return tool.Request{Call: types.ToolCall{
		ID: "call_1", Name: name, Input: json.RawMessage(input)}}
}

func TestEditToolsDeniedWithoutWritePermission(t *testing.T) {
	exec := workspace(t)
	m := editManager(t, exec, tool.NewPermissionPolicy(tool.PermissionRead))

	res := m.Execute(context.Background(), editCall("write_file",
		`{"path":"denied.txt","content":"x"}`))
	if kind := types.KindOf(res.Err); kind != types.ErrorKindPermissionDenied {
		t.Fatalf("error kind = %q (%v), want %q", kind, res.Err, types.ErrorKindPermissionDenied)
	}
	if _, err := os.Stat(filepath.Join(exec.WorkDir, "denied.txt")); !os.IsNotExist(err) {
		t.Error("file was created despite the permission denial")
	}
}

func TestEditToolsDryRunThroughManager(t *testing.T) {
	exec := workspace(t)
	m := editManager(t, exec, tool.NewPermissionPolicy(tool.PermissionWrite))

	req := editCall("create_file", `{"path":"dry.txt","content":"x"}`)
	req.DryRun = true
	res := m.Execute(context.Background(), req)
	if res.Err != nil {
		t.Fatalf("dry run failed: %v", res.Err)
	}
	if !strings.Contains(res.Content, "dry run") {
		t.Errorf("content = %q, want a dry-run report", res.Content)
	}
	if _, err := os.Stat(filepath.Join(exec.WorkDir, "dry.txt")); !os.IsNotExist(err) {
		t.Error("dry run created the file")
	}
}

func TestCreateFileToolPreviewAndApply(t *testing.T) {
	exec := workspace(t)
	m := editManager(t, exec, tool.NewPermissionPolicy(tool.PermissionWrite))
	ctx := context.Background()

	res := m.Execute(ctx, editCall("create_file",
		`{"path":"new.txt","content":"hello\n","preview":true}`))
	if res.Err != nil {
		t.Fatalf("preview failed: %v", res.Err)
	}
	if !strings.Contains(res.Content, "preview") || !strings.Contains(res.Content, "+hello") {
		t.Errorf("preview content = %q, want a preview with the diff", res.Content)
	}
	if _, err := os.Stat(filepath.Join(exec.WorkDir, "new.txt")); !os.IsNotExist(err) {
		t.Error("preview created the file")
	}

	res = m.Execute(ctx, editCall("create_file", `{"path":"new.txt","content":"hello\n"}`))
	if res.Err != nil {
		t.Fatalf("apply failed: %v", res.Err)
	}
	data, err := os.ReadFile(filepath.Join(exec.WorkDir, "new.txt"))
	if err != nil || string(data) != "hello\n" {
		t.Errorf("content = %q (%v), want %q", data, err, "hello\n")
	}
}

func TestReplaceTextToolThroughManager(t *testing.T) {
	exec := workspace(t)
	m := editManager(t, exec, tool.NewPermissionPolicy(tool.PermissionWrite))

	res := m.Execute(context.Background(), editCall("replace_text",
		`{"path":"notes.txt","target":"world","replacement":"there"}`))
	if res.Err != nil {
		t.Fatalf("Execute: %v", res.Err)
	}
	if !strings.Contains(res.Content, "replaced 1 occurrence(s) in notes.txt") {
		t.Errorf("content = %q, want the replacement summary", res.Content)
	}
	data, err := os.ReadFile(filepath.Join(exec.WorkDir, "notes.txt"))
	if err != nil || string(data) != "hello there" {
		t.Errorf("content = %q (%v), want %q", data, err, "hello there")
	}
}

func TestInsertAndDeleteToolsThroughManager(t *testing.T) {
	exec := workspace(t)
	m := editManager(t, exec, tool.NewPermissionPolicy(tool.PermissionWrite))
	ctx := context.Background()

	res := m.Execute(ctx, editCall("insert_text",
		`{"path":"notes.txt","line":1,"text":"header"}`))
	if res.Err != nil {
		t.Fatalf("insert: %v", res.Err)
	}
	data, _ := os.ReadFile(filepath.Join(exec.WorkDir, "notes.txt"))
	if string(data) != "header\nhello world" {
		t.Errorf("after insert = %q", data)
	}

	res = m.Execute(ctx, editCall("delete_text",
		`{"path":"notes.txt","target":"header\n"}`))
	if res.Err != nil {
		t.Fatalf("delete: %v", res.Err)
	}
	data, _ = os.ReadFile(filepath.Join(exec.WorkDir, "notes.txt"))
	if string(data) != "hello world" {
		t.Errorf("after delete = %q", data)
	}
}

func TestRenameFileToolThroughManager(t *testing.T) {
	exec := workspace(t)
	m := editManager(t, exec, tool.NewPermissionPolicy(tool.PermissionWrite))

	res := m.Execute(context.Background(), editCall("rename_file",
		`{"path":"notes.txt","new_path":"sub/renamed.txt"}`))
	if res.Err != nil {
		t.Fatalf("Execute: %v", res.Err)
	}
	data, err := os.ReadFile(filepath.Join(exec.WorkDir, "sub", "renamed.txt"))
	if err != nil || string(data) != "hello world" {
		t.Errorf("renamed content = %q (%v)", data, err)
	}
	if _, err := os.Stat(filepath.Join(exec.WorkDir, "notes.txt")); !os.IsNotExist(err) {
		t.Error("source still exists after rename")
	}
}

func TestEditToolsRefusePathsOutsideWorkDir(t *testing.T) {
	exec := workspace(t)
	m := editManager(t, exec, tool.NewPermissionPolicy(tool.PermissionWrite))
	ctx := context.Background()

	cases := []struct {
		name  string
		input string
	}{
		{"write_file", `{"path":"../escape.txt","content":"x"}`},
		{"create_file", `{"path":"/tmp/escape.txt","content":"x"}`},
		{"rename_file", `{"path":"notes.txt","new_path":"../escape.txt"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := m.Execute(ctx, editCall(tc.name, tc.input))
			if kind := types.KindOf(res.Err); kind != types.ErrorKindPermissionDenied {
				t.Errorf("error kind = %q (%v), want %q", kind, res.Err, types.ErrorKindPermissionDenied)
			}
		})
	}
}

func TestWriteFileToolConflictDetection(t *testing.T) {
	exec := workspace(t)
	m := editManager(t, exec, tool.NewPermissionPolicy(tool.PermissionWrite))
	ctx := context.Background()

	// A preview reports the base hash of the current content.
	res := m.Execute(ctx, editCall("write_file",
		`{"path":"notes.txt","content":"new\n","preview":true}`))
	if res.Err != nil {
		t.Fatalf("preview: %v", res.Err)
	}
	var baseHash string
	for _, line := range strings.Split(res.Content, "\n") {
		if hash, ok := strings.CutPrefix(line, "base_hash: "); ok {
			baseHash = hash
		}
	}
	if baseHash == "" {
		t.Fatalf("preview content %q carries no base_hash", res.Content)
	}

	// The file changes underneath; the guarded apply must refuse.
	if err := os.WriteFile(filepath.Join(exec.WorkDir, "notes.txt"), []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	res = m.Execute(ctx, editCall("write_file",
		`{"path":"notes.txt","content":"new\n","base_hash":"`+baseHash+`"}`))
	if kind := types.KindOf(res.Err); kind != types.ErrorKindConflict {
		t.Fatalf("error kind = %q (%v), want %q", kind, res.Err, types.ErrorKindConflict)
	}
	data, _ := os.ReadFile(filepath.Join(exec.WorkDir, "notes.txt"))
	if string(data) != "changed" {
		t.Errorf("conflicting write modified the file: %q", data)
	}
}

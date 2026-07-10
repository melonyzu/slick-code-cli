package builtin

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/melonyzu/slick-code-cli/internal/tool"
	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// workspace builds a temp working directory with a known layout:
//
//	notes.txt      ("hello world")
//	sub/           (directory)
//	sub/inner.txt  ("inner")
func workspace(t *testing.T) tool.ExecContext {
	t.Helper()
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("hello world"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "inner.txt"), []byte("inner"), 0o600); err != nil {
		t.Fatal(err)
	}
	return tool.ExecContext{WorkDir: dir}
}

func TestReadFile(t *testing.T) {
	exec := workspace(t)
	rf := NewReadFile()

	got, err := rf.Execute(context.Background(), exec, json.RawMessage(`{"path":"notes.txt"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got != "hello world" {
		t.Errorf("content = %q, want %q", got, "hello world")
	}

	got, err = rf.Execute(context.Background(), exec, json.RawMessage(`{"path":"sub/inner.txt"}`))
	if err != nil {
		t.Fatalf("Execute nested: %v", err)
	}
	if got != "inner" {
		t.Errorf("nested content = %q, want %q", got, "inner")
	}
}

func TestReadFileErrors(t *testing.T) {
	exec := workspace(t)
	rf := NewReadFile()

	cases := []struct {
		name  string
		input string
		kind  types.ErrorKind
	}{
		{"missing file", `{"path":"absent.txt"}`, types.ErrorKindValidation},
		{"directory", `{"path":"sub"}`, types.ErrorKindValidation},
		{"empty path", `{}`, types.ErrorKindValidation},
		{"malformed input", `{"path":42}`, types.ErrorKindValidation},
		{"escapes workdir", `{"path":"../outside.txt"}`, types.ErrorKindPermissionDenied},
		{"absolute outside", `{"path":"/etc/hostname"}`, types.ErrorKindPermissionDenied},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := rf.Execute(context.Background(), exec, json.RawMessage(tc.input))
			if err == nil {
				t.Fatal("Execute succeeded, want error")
			}
			if kind := types.KindOf(err); kind != tc.kind {
				t.Errorf("error kind = %q (%v), want %q", kind, err, tc.kind)
			}
		})
	}
}

func TestReadFileRejectsOversizedFile(t *testing.T) {
	exec := workspace(t)
	big := make([]byte, maxReadBytes+1)
	if err := os.WriteFile(filepath.Join(exec.WorkDir, "big.bin"), big, 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := NewReadFile().Execute(context.Background(), exec, json.RawMessage(`{"path":"big.bin"}`))
	if kind := types.KindOf(err); kind != types.ErrorKindValidation {
		t.Errorf("error kind = %q (%v), want %q", kind, err, types.ErrorKindValidation)
	}
}

func TestListDirectory(t *testing.T) {
	exec := workspace(t)
	ld := NewListDirectory()

	got, err := ld.Execute(context.Background(), exec, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	want := "notes.txt\nsub/"
	if got != want {
		t.Errorf("listing = %q, want %q", got, want)
	}

	got, err = ld.Execute(context.Background(), exec, json.RawMessage(`{"path":"sub"}`))
	if err != nil {
		t.Fatalf("Execute sub: %v", err)
	}
	if got != "inner.txt" {
		t.Errorf("sub listing = %q, want %q", got, "inner.txt")
	}
}

func TestListDirectoryEmpty(t *testing.T) {
	exec := tool.ExecContext{WorkDir: t.TempDir()}

	got, err := NewListDirectory().Execute(context.Background(), exec, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(got, "empty") {
		t.Errorf("empty listing = %q, want it to say the directory is empty", got)
	}
}

func TestListDirectoryErrors(t *testing.T) {
	exec := workspace(t)
	ld := NewListDirectory()

	cases := []struct {
		name  string
		input string
		kind  types.ErrorKind
	}{
		{"missing directory", `{"path":"absent"}`, types.ErrorKindValidation},
		{"escapes workdir", `{"path":".."}`, types.ErrorKindPermissionDenied},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ld.Execute(context.Background(), exec, json.RawMessage(tc.input))
			if err == nil {
				t.Fatal("Execute succeeded, want error")
			}
			if kind := types.KindOf(err); kind != tc.kind {
				t.Errorf("error kind = %q (%v), want %q", kind, err, tc.kind)
			}
		})
	}
}

// TestDefinitionsAreValid ensures each built-in advertises a complete
// definition with a parseable JSON Schema and the permission matching
// what it does.
func TestDefinitionsAreValid(t *testing.T) {
	engine := testEngine()
	byPermission := map[tool.Permission][]tool.Tool{
		tool.PermissionRead: {
			NewReadFile(), NewListDirectory(), NewProjectContext(nil),
			NewGitStatus(nil), NewGitDiff(nil),
		},
		tool.PermissionWrite: {
			NewCreateFile(engine), NewWriteFile(engine), NewReplaceText(engine),
			NewInsertText(engine), NewDeleteText(engine), NewRenameFile(engine),
			NewGitCommit(nil), NewGitRestore(nil), NewGitCheckout(nil), NewGitBranch(nil),
		},
	}
	for want, tools := range byPermission {
		for _, tl := range tools {
			def := tl.Definition()
			if def.Name == "" || def.Description == "" {
				t.Errorf("%T definition is missing name or description", tl)
			}
			var schema map[string]any
			if err := json.Unmarshal(def.InputSchema, &schema); err != nil {
				t.Errorf("%s input schema is not valid JSON: %v", def.Name, err)
			}
			if tl.Permission() != want {
				t.Errorf("%s permission = %q, want %q", def.Name, tl.Permission(), want)
			}
		}
	}
}

package edit

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/melonyzu/slick-code-cli/pkg/types"
)

func newTestEngine() *Engine {
	return NewEngine(slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func writeFile(t *testing.T, dir, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestApplyCreate(t *testing.T) {
	e, dir := newTestEngine(), t.TempDir()
	path := filepath.Join(dir, "sub", "new.txt")

	res, err := e.Apply(context.Background(), Request{
		Op: OpCreate, Path: path, Display: "sub/new.txt", Content: "hello\n"})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got := readFile(t, path); string(got) != "hello\n" {
		t.Errorf("content = %q, want %q", got, "hello\n")
	}
	if !res.Created || !res.Applied || res.Rollback == nil {
		t.Errorf("Result = %+v, want created, applied, with rollback", res)
	}
	if !strings.Contains(res.Diff, "/dev/null") || !strings.Contains(res.Diff, "+hello") {
		t.Errorf("Diff = %q, want a creation diff", res.Diff)
	}
}

func TestCreateExistingFileConflicts(t *testing.T) {
	e, dir := newTestEngine(), t.TempDir()
	path := writeFile(t, dir, "a.txt", []byte("x"))

	_, err := e.Apply(context.Background(), Request{Op: OpCreate, Path: path, Content: "y"})
	if kind := types.KindOf(err); kind != types.ErrorKindConflict {
		t.Errorf("error kind = %q (%v), want %q", kind, err, types.ErrorKindConflict)
	}
	if got := readFile(t, path); string(got) != "x" {
		t.Errorf("file was modified by a refused create: %q", got)
	}
}

func TestPreviewLeavesFileUntouched(t *testing.T) {
	e, dir := newTestEngine(), t.TempDir()
	path := writeFile(t, dir, "a.txt", []byte("one\ntwo\n"))

	res, err := e.Preview(context.Background(), Request{
		Op: OpReplace, Path: path, Target: "two", Replacement: "three"})
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if got := readFile(t, path); string(got) != "one\ntwo\n" {
		t.Errorf("preview modified the file: %q", got)
	}
	if res.Applied || res.Rollback != nil {
		t.Errorf("Result = %+v, want not applied and no rollback", res)
	}
	if !strings.Contains(res.Diff, "-two") || !strings.Contains(res.Diff, "+three") {
		t.Errorf("Diff = %q, want the replacement shown", res.Diff)
	}
	if res.OldHash == "" || res.NewHash == "" || res.OldHash == res.NewHash {
		t.Errorf("hashes = %q -> %q, want distinct non-empty hashes", res.OldHash, res.NewHash)
	}
}

func TestReplaceOperations(t *testing.T) {
	ctx := context.Background()

	t.Run("single occurrence", func(t *testing.T) {
		e, dir := newTestEngine(), t.TempDir()
		path := writeFile(t, dir, "a.txt", []byte("aaa bbb ccc\n"))
		res, err := e.Apply(ctx, Request{Op: OpReplace, Path: path, Target: "bbb", Replacement: "BBB"})
		if err != nil {
			t.Fatalf("Apply: %v", err)
		}
		if got := readFile(t, path); string(got) != "aaa BBB ccc\n" {
			t.Errorf("content = %q", got)
		}
		if res.Occurrences != 1 {
			t.Errorf("Occurrences = %d, want 1", res.Occurrences)
		}
	})

	t.Run("ambiguous without all", func(t *testing.T) {
		e, dir := newTestEngine(), t.TempDir()
		path := writeFile(t, dir, "a.txt", []byte("x x\n"))
		_, err := e.Apply(ctx, Request{Op: OpReplace, Path: path, Target: "x", Replacement: "y"})
		if kind := types.KindOf(err); kind != types.ErrorKindValidation {
			t.Errorf("error kind = %q (%v), want validation", kind, err)
		}
	})

	t.Run("all occurrences", func(t *testing.T) {
		e, dir := newTestEngine(), t.TempDir()
		path := writeFile(t, dir, "a.txt", []byte("x x x\n"))
		res, err := e.Apply(ctx, Request{Op: OpReplace, Path: path, Target: "x", Replacement: "y", All: true})
		if err != nil {
			t.Fatalf("Apply: %v", err)
		}
		if got := readFile(t, path); string(got) != "y y y\n" {
			t.Errorf("content = %q", got)
		}
		if res.Occurrences != 3 {
			t.Errorf("Occurrences = %d, want 3", res.Occurrences)
		}
	})

	t.Run("target not found", func(t *testing.T) {
		e, dir := newTestEngine(), t.TempDir()
		path := writeFile(t, dir, "a.txt", []byte("abc\n"))
		_, err := e.Apply(ctx, Request{Op: OpReplace, Path: path, Target: "zzz", Replacement: "y"})
		if kind := types.KindOf(err); kind != types.ErrorKindValidation {
			t.Errorf("error kind = %q (%v), want validation", kind, err)
		}
	})
}

func TestDeleteText(t *testing.T) {
	e, dir := newTestEngine(), t.TempDir()
	path := writeFile(t, dir, "a.txt", []byte("keep DROP keep DROP \n"))

	res, err := e.Apply(context.Background(), Request{Op: OpDelete, Path: path, Target: "DROP ", All: true})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got := readFile(t, path); string(got) != "keep keep \n" {
		t.Errorf("content = %q", got)
	}
	if res.Occurrences != 2 {
		t.Errorf("Occurrences = %d, want 2", res.Occurrences)
	}
}

func TestInsertText(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name    string
		initial string
		line    int
		text    string
		want    string
	}{
		{"before first line", "b\nc\n", 1, "a", "a\nb\nc\n"},
		{"middle", "a\nc\n", 2, "b", "a\nb\nc\n"},
		{"append with trailing newline", "a\n", 2, "b", "a\nb\n"},
		{"append preserves missing final newline", "a", 2, "b", "a\nb"},
		{"empty file", "", 1, "a", "a\n"},
		{"multi-line block", "a\nd\n", 2, "b\nc", "a\nb\nc\nd\n"},
		{"crlf file uses crlf", "a\r\nc\r\n", 2, "b", "a\r\nb\r\nc\r\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e, dir := newTestEngine(), t.TempDir()
			path := writeFile(t, dir, "a.txt", []byte(tc.initial))
			if _, err := e.Apply(ctx, Request{Op: OpInsert, Path: path, Line: tc.line, Text: tc.text}); err != nil {
				t.Fatalf("Apply: %v", err)
			}
			if got := readFile(t, path); string(got) != tc.want {
				t.Errorf("content = %q, want %q", got, tc.want)
			}
		})
	}

	t.Run("line out of range", func(t *testing.T) {
		e, dir := newTestEngine(), t.TempDir()
		path := writeFile(t, dir, "a.txt", []byte("a\n"))
		_, err := e.Apply(ctx, Request{Op: OpInsert, Path: path, Line: 5, Text: "x"})
		if kind := types.KindOf(err); kind != types.ErrorKindValidation {
			t.Errorf("error kind = %q (%v), want validation", kind, err)
		}
	})
}

func TestReplacePreservesCRLF(t *testing.T) {
	e, dir := newTestEngine(), t.TempDir()
	path := writeFile(t, dir, "a.txt", []byte("one\r\ntwo\r\n"))

	if _, err := e.Apply(context.Background(), Request{
		Op: OpReplace, Path: path, Target: "two", Replacement: "three"}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got := readFile(t, path); string(got) != "one\r\nthree\r\n" {
		t.Errorf("content = %q, want CRLF endings preserved", got)
	}
}

func TestEncodingPreservation(t *testing.T) {
	ctx := context.Background()

	t.Run("utf-8 bom", func(t *testing.T) {
		e, dir := newTestEngine(), t.TempDir()
		path := writeFile(t, dir, "a.txt", append(append([]byte{}, bomUTF8...), "héllo\n"...))
		if _, err := e.Apply(ctx, Request{Op: OpReplace, Path: path, Target: "héllo", Replacement: "wörld"}); err != nil {
			t.Fatalf("Apply: %v", err)
		}
		got := readFile(t, path)
		if !bytes.HasPrefix(got, bomUTF8) {
			t.Error("UTF-8 BOM was lost")
		}
		if string(got[len(bomUTF8):]) != "wörld\n" {
			t.Errorf("content = %q", got)
		}
	})

	t.Run("utf-16le round trip", func(t *testing.T) {
		e, dir := newTestEngine(), t.TempDir()
		original := encode("hello\nworld\n", encUTF16LE)
		path := writeFile(t, dir, "a.txt", original)
		if _, err := e.Apply(ctx, Request{Op: OpReplace, Path: path, Target: "world", Replacement: "there"}); err != nil {
			t.Fatalf("Apply: %v", err)
		}
		if got, want := readFile(t, path), encode("hello\nthere\n", encUTF16LE); !bytes.Equal(got, want) {
			t.Errorf("bytes = %v, want UTF-16LE with BOM preserved (%v)", got, want)
		}
	})

	t.Run("binary refused", func(t *testing.T) {
		e, dir := newTestEngine(), t.TempDir()
		path := writeFile(t, dir, "a.bin", []byte{0x01, 0x00, 0x02, 0xFF})
		_, err := e.Apply(ctx, Request{Op: OpReplace, Path: path, Target: "x", Replacement: "y"})
		if kind := types.KindOf(err); kind != types.ErrorKindValidation {
			t.Errorf("error kind = %q (%v), want validation", kind, err)
		}
	})
}

func TestConflictDetection(t *testing.T) {
	e, dir := newTestEngine(), t.TempDir()
	path := writeFile(t, dir, "a.txt", []byte("one\n"))
	ctx := context.Background()

	res, err := e.Preview(ctx, Request{Op: OpReplace, Path: path, Target: "one", Replacement: "two"})
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}

	// The file changes between the preview and the apply.
	writeFile(t, dir, "a.txt", []byte("surprise\n"))

	_, err = e.Apply(ctx, Request{
		Op: OpReplace, Path: path, Target: "one", Replacement: "two", BaseHash: res.OldHash})
	if kind := types.KindOf(err); kind != types.ErrorKindConflict {
		t.Fatalf("error kind = %q (%v), want %q", kind, err, types.ErrorKindConflict)
	}
	if got := readFile(t, path); string(got) != "surprise\n" {
		t.Errorf("conflicting apply modified the file: %q", got)
	}

	// With the current hash the same edit goes through.
	writeFile(t, dir, "a.txt", []byte("one\n"))
	cur, err := e.Preview(ctx, Request{Op: OpReplace, Path: path, Target: "one", Replacement: "two"})
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if _, err := e.Apply(ctx, Request{
		Op: OpReplace, Path: path, Target: "one", Replacement: "two", BaseHash: cur.OldHash}); err != nil {
		t.Fatalf("Apply with matching base hash: %v", err)
	}
	if got := readFile(t, path); string(got) != "two\n" {
		t.Errorf("content = %q, want %q", got, "two\n")
	}
}

func TestRollbackRestoresPriorState(t *testing.T) {
	ctx := context.Background()

	t.Run("replace", func(t *testing.T) {
		e, dir := newTestEngine(), t.TempDir()
		path := writeFile(t, dir, "a.txt", []byte("one\r\ntwo\r\n"))
		res, err := e.Apply(ctx, Request{Op: OpReplace, Path: path, Target: "two", Replacement: "three"})
		if err != nil {
			t.Fatalf("Apply: %v", err)
		}
		if err := e.Rollback(ctx, res.Rollback); err != nil {
			t.Fatalf("Rollback: %v", err)
		}
		if got := readFile(t, path); string(got) != "one\r\ntwo\r\n" {
			t.Errorf("content = %q, want the original bytes back", got)
		}
	})

	t.Run("create removes the file", func(t *testing.T) {
		e, dir := newTestEngine(), t.TempDir()
		path := filepath.Join(dir, "new.txt")
		res, err := e.Apply(ctx, Request{Op: OpCreate, Path: path, Content: "x"})
		if err != nil {
			t.Fatalf("Apply: %v", err)
		}
		if err := e.Rollback(ctx, res.Rollback); err != nil {
			t.Fatalf("Rollback: %v", err)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("file still exists after rolling back its creation")
		}
	})

	t.Run("undo pops the journal", func(t *testing.T) {
		e, dir := newTestEngine(), t.TempDir()
		path := writeFile(t, dir, "a.txt", []byte("v1"))
		if _, err := e.Apply(ctx, Request{Op: OpWrite, Path: path, Content: "v2"}); err != nil {
			t.Fatalf("Apply: %v", err)
		}
		if _, err := e.Undo(ctx); err != nil {
			t.Fatalf("Undo: %v", err)
		}
		if got := readFile(t, path); string(got) != "v1" {
			t.Errorf("content = %q, want %q", got, "v1")
		}
		if _, err := e.Undo(ctx); types.KindOf(err) != types.ErrorKindValidation {
			t.Errorf("second Undo error = %v, want validation (empty journal)", err)
		}
	})
}

func TestRollbackConflictsWhenFileChangedAfterEdit(t *testing.T) {
	e, dir := newTestEngine(), t.TempDir()
	path := writeFile(t, dir, "a.txt", []byte("one\n"))
	ctx := context.Background()

	res, err := e.Apply(ctx, Request{Op: OpWrite, Path: path, Content: "two\n"})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	writeFile(t, dir, "a.txt", []byte("later work\n"))

	err = e.Rollback(ctx, res.Rollback)
	if kind := types.KindOf(err); kind != types.ErrorKindConflict {
		t.Fatalf("error kind = %q (%v), want %q", kind, err, types.ErrorKindConflict)
	}
	if got := readFile(t, path); string(got) != "later work\n" {
		t.Errorf("rollback clobbered later modifications: %q", got)
	}
}

func TestRenameOperations(t *testing.T) {
	ctx := context.Background()

	t.Run("renames and rolls back", func(t *testing.T) {
		e, dir := newTestEngine(), t.TempDir()
		oldPath := writeFile(t, dir, "old.txt", []byte("content"))
		newPath := filepath.Join(dir, "sub", "new.txt")

		res, err := e.Apply(ctx, Request{Op: OpRename, Path: oldPath, NewPath: newPath})
		if err != nil {
			t.Fatalf("Apply: %v", err)
		}
		if got := readFile(t, newPath); string(got) != "content" {
			t.Errorf("renamed content = %q", got)
		}
		if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
			t.Error("source still exists after rename")
		}

		if err := e.Rollback(ctx, res.Rollback); err != nil {
			t.Fatalf("Rollback: %v", err)
		}
		if got := readFile(t, oldPath); string(got) != "content" {
			t.Errorf("rolled-back content = %q", got)
		}
	})

	t.Run("existing destination conflicts", func(t *testing.T) {
		e, dir := newTestEngine(), t.TempDir()
		oldPath := writeFile(t, dir, "old.txt", []byte("a"))
		newPath := writeFile(t, dir, "new.txt", []byte("b"))

		_, err := e.Apply(ctx, Request{Op: OpRename, Path: oldPath, NewPath: newPath})
		if kind := types.KindOf(err); kind != types.ErrorKindConflict {
			t.Errorf("error kind = %q (%v), want conflict", kind, err)
		}
		if got := readFile(t, newPath); string(got) != "b" {
			t.Errorf("destination was overwritten: %q", got)
		}
	})

	t.Run("missing source", func(t *testing.T) {
		e, dir := newTestEngine(), t.TempDir()
		_, err := e.Apply(ctx, Request{
			Op: OpRename, Path: filepath.Join(dir, "absent.txt"), NewPath: filepath.Join(dir, "x.txt")})
		if kind := types.KindOf(err); kind != types.ErrorKindValidation {
			t.Errorf("error kind = %q (%v), want validation", kind, err)
		}
	})
}

func TestAtomicWrites(t *testing.T) {
	e, dir := newTestEngine(), t.TempDir()
	ctx := context.Background()

	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte("v1"), 0o640); err != nil {
		t.Fatal(err)
	}

	if _, err := e.Apply(ctx, Request{Op: OpWrite, Path: path, Content: "v2"}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Errorf("mode = %v, want the original 0640 preserved", info.Mode().Perm())
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp") {
			t.Errorf("temporary file %s left behind", entry.Name())
		}
	}
}

func TestCanceledContextAbortsEdit(t *testing.T) {
	e, dir := newTestEngine(), t.TempDir()
	path := writeFile(t, dir, "a.txt", []byte("one\n"))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := e.Apply(ctx, Request{Op: OpWrite, Path: path, Content: "two\n"}); err == nil {
		t.Fatal("Apply succeeded under a canceled context")
	}
	if got := readFile(t, path); string(got) != "one\n" {
		t.Errorf("canceled apply modified the file: %q", got)
	}
}

func TestValidation(t *testing.T) {
	cases := []struct {
		name string
		req  Request
	}{
		{"empty path", Request{Op: OpWrite}},
		{"unknown op", Request{Op: "copy", Path: "/tmp/x"}},
		{"replace without target", Request{Op: OpReplace, Path: "/tmp/x"}},
		{"insert without text", Request{Op: OpInsert, Path: "/tmp/x", Line: 1}},
		{"insert line zero", Request{Op: OpInsert, Path: "/tmp/x", Line: 0, Text: "y"}},
		{"rename without destination", Request{Op: OpRename, Path: "/tmp/x"}},
		{"rename to itself", Request{Op: OpRename, Path: "/tmp/x", NewPath: "/tmp/x"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.req.Validate()
			if kind := types.KindOf(err); kind != types.ErrorKindValidation {
				t.Errorf("error kind = %q (%v), want validation", kind, err)
			}
		})
	}
}

func TestUnifiedDiff(t *testing.T) {
	t.Run("equal texts produce no diff", func(t *testing.T) {
		if d := unifiedDiff("a.txt", "same\n", "same\n", true); d != "" {
			t.Errorf("diff = %q, want empty", d)
		}
	})

	t.Run("hunks carry headers and context", func(t *testing.T) {
		oldText := "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n"
		newText := "1\n2\n3\n4\nX\n6\n7\n8\n9\n10\n"
		d := unifiedDiff("a.txt", oldText, newText, true)
		for _, want := range []string{"--- a/a.txt", "+++ b/a.txt", "@@ -2,7 +2,7 @@", "-5", "+X", " 4", " 6"} {
			if !strings.Contains(d, want) {
				t.Errorf("diff missing %q:\n%s", want, d)
			}
		}
		if strings.Contains(d, " 1\n") {
			t.Errorf("diff includes line 1, beyond the context window:\n%s", d)
		}
	})

	t.Run("distant changes split into separate hunks", func(t *testing.T) {
		var oldLines []string
		for i := 1; i <= 30; i++ {
			oldLines = append(oldLines, strings.Repeat("l", i))
		}
		newLines := append([]string(nil), oldLines...)
		newLines[0] = "first"
		newLines[29] = "last"
		d := unifiedDiff("a.txt",
			strings.Join(oldLines, "\n")+"\n", strings.Join(newLines, "\n")+"\n", true)
		if got := strings.Count(d, "@@ -"); got != 2 {
			t.Errorf("hunks = %d, want 2:\n%s", got, d)
		}
	})
}

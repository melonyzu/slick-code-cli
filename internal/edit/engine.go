package edit

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// maxHistory bounds the engine's in-memory rollback journal.
const maxHistory = 50

// Engine plans and applies file edits. Every edit is planned fully in
// memory — validation, conflict detection, encoding and line-ending
// preservation, diff rendering — before a single byte reaches disk, so
// a failed edit never leaves a file half-modified. Applied edits are
// committed atomically and recorded in a bounded rollback journal.
//
// An Engine is safe for concurrent use; the journal is the only shared
// state.
type Engine struct {
	logger *slog.Logger

	mu      sync.Mutex
	history []*Rollback
}

// NewEngine returns an Engine logging through logger.
func NewEngine(logger *slog.Logger) *Engine {
	return &Engine{logger: logger}
}

// Preview plans req and reports what applying it would do — the diff,
// occurrence counts, and content hashes — without touching the
// filesystem. The returned Result's OldHash can be passed as a later
// request's BaseHash to guarantee the file hasn't changed in between.
func (e *Engine) Preview(ctx context.Context, req Request) (*Result, error) {
	p, err := e.plan(ctx, req)
	if err != nil {
		return nil, err
	}
	e.logger.Debug("edit previewed",
		"op", string(req.Op), "path", req.display(), "bytes", len(p.newRaw))
	return p.result(false), nil
}

// Apply plans req and commits it to disk atomically. The returned
// Result carries a Rollback token restoring the prior state; the edit
// is also recorded in the engine's journal for Undo.
func (e *Engine) Apply(ctx context.Context, req Request) (*Result, error) {
	start := time.Now()
	p, err := e.plan(ctx, req)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := e.commit(p); err != nil {
		return nil, err
	}

	res := p.result(true)
	res.Rollback = &Rollback{
		Op:      req.Op,
		Path:    req.Path,
		NewPath: req.NewPath,
		Existed: p.existed,
		Content: p.oldRaw,
		Mode:    p.mode,
		NewHash: res.NewHash,
	}
	e.record(res.Rollback)

	e.logger.Info("edit applied",
		"op", string(req.Op), "path", req.display(),
		"created", res.Created, "occurrences", res.Occurrences,
		"bytes", res.Bytes, "old_hash", res.OldHash, "new_hash", res.NewHash,
		"duration", time.Since(start))
	return res, nil
}

// Rollback restores the state captured in rb. It refuses with a
// conflict error if the file changed after the edit, so a rollback can
// never destroy later work.
func (e *Engine) Rollback(ctx context.Context, rb *Rollback) error {
	if rb == nil {
		return types.NewError(types.ErrorKindValidation, "edit: nothing to roll back")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	if rb.Op == OpRename {
		if err := verifyUnchanged(rb.NewPath, rb.NewHash); err != nil {
			return err
		}
		if _, err := os.Stat(rb.Path); err == nil {
			return types.NewError(types.ErrorKindConflict, fmt.Sprintf(
				"edit: cannot roll back rename; %s exists again", rb.Path))
		}
		if err := os.Rename(rb.NewPath, rb.Path); err != nil {
			return types.WrapError(types.ErrorKindInternal, "edit: roll back rename", err)
		}
	} else {
		if err := verifyUnchanged(rb.Path, rb.NewHash); err != nil {
			return err
		}
		if rb.Existed {
			if err := writeAtomic(rb.Path, rb.Content, rb.Mode); err != nil {
				return err
			}
		} else if err := os.Remove(rb.Path); err != nil {
			return types.WrapError(types.ErrorKindInternal, "edit: remove "+rb.Path, err)
		}
	}

	e.logger.Info("edit rolled back", "op", string(rb.Op), "path", rb.Path)
	return nil
}

// Undo rolls back the most recently applied edit in the journal. The
// entry is consumed even when the rollback fails, because a conflicting
// rollback will never become safe later.
func (e *Engine) Undo(ctx context.Context) (*Rollback, error) {
	e.mu.Lock()
	var rb *Rollback
	if n := len(e.history); n > 0 {
		rb = e.history[n-1]
		e.history = e.history[:n-1]
	}
	e.mu.Unlock()

	if rb == nil {
		return nil, types.NewError(types.ErrorKindValidation, "edit: no edits to roll back")
	}
	return rb, e.Rollback(ctx, rb)
}

// record appends rb to the journal, dropping the oldest entry beyond
// maxHistory.
func (e *Engine) record(rb *Rollback) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.history = append(e.history, rb)
	if len(e.history) > maxHistory {
		e.history = e.history[len(e.history)-maxHistory:]
	}
}

// verifyUnchanged confirms the file at path still holds the content
// hash the edit produced.
func verifyUnchanged(path, wantHash string) error {
	cur, err := os.ReadFile(path)
	switch {
	case os.IsNotExist(err):
		return types.NewError(types.ErrorKindConflict,
			fmt.Sprintf("edit: cannot roll back; %s no longer exists", path))
	case err != nil:
		return types.WrapError(types.ErrorKindInternal, "edit: read "+path, err)
	case hashBytes(cur) != wantHash:
		return types.NewError(types.ErrorKindConflict, fmt.Sprintf(
			"edit: cannot roll back; %s was modified after the edit", path))
	}
	return nil
}

// plan holds a fully computed edit, ready to commit.
type plan struct {
	req         Request
	existed     bool
	mode        os.FileMode
	oldRaw      []byte
	newRaw      []byte
	occurrences int
	diff        string
}

// plan validates req, reads the current file state, detects conflicts,
// and computes the new content entirely in memory.
func (e *Engine) plan(ctx context.Context, req Request) (*plan, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	p := &plan{req: req, mode: 0o644}
	info, err := os.Stat(req.Path)
	switch {
	case err == nil:
		if info.IsDir() {
			return nil, types.NewError(types.ErrorKindValidation,
				fmt.Sprintf("edit: %s is a directory", req.display()))
		}
		if info.Size() > maxEditBytes {
			return nil, types.NewError(types.ErrorKindValidation, fmt.Sprintf(
				"edit: %s is %d bytes, over the %d byte limit", req.display(), info.Size(), maxEditBytes))
		}
		p.existed = true
		p.mode = info.Mode().Perm()
		if p.oldRaw, err = os.ReadFile(req.Path); err != nil {
			return nil, types.WrapError(types.ErrorKindInternal, "edit: read "+req.display(), err)
		}
	case !os.IsNotExist(err):
		return nil, types.WrapError(types.ErrorKindInternal, "edit: stat "+req.display(), err)
	}

	if req.BaseHash != "" {
		switch {
		case !p.existed:
			return nil, types.NewError(types.ErrorKindConflict, fmt.Sprintf(
				"edit: %s no longer exists; re-read the file and retry", req.display()))
		case hashBytes(p.oldRaw) != req.BaseHash:
			return nil, types.NewError(types.ErrorKindConflict, fmt.Sprintf(
				"edit: %s was modified since it was read; re-read the file and retry", req.display()))
		}
	}

	if err := p.compute(); err != nil {
		return nil, err
	}
	return p, nil
}

// compute derives the plan's new content and diff for its operation.
func (p *plan) compute() error {
	req := p.req

	// Decode the current content once for every operation that reads
	// it; the same detected encoding is used to write the result back.
	var oldText string
	enc := encUTF8
	if p.existed {
		var err error
		if oldText, enc, err = decode(p.oldRaw); err != nil {
			return err
		}
	}

	var newText string
	switch req.Op {
	case OpCreate:
		if p.existed {
			return types.NewError(types.ErrorKindConflict, fmt.Sprintf(
				"edit: %s already exists; use the write operation to overwrite it", req.display()))
		}
		newText = req.Content
	case OpWrite:
		newText = req.Content
	case OpReplace, OpDelete:
		if err := p.requireExisting(); err != nil {
			return err
		}
		replacement := req.Replacement
		if req.Op == OpDelete {
			replacement = ""
		}
		var err error
		if newText, p.occurrences, err = replaceText(oldText, req.Target, replacement, req.All); err != nil {
			return err
		}
	case OpInsert:
		if err := p.requireExisting(); err != nil {
			return err
		}
		var err error
		if newText, err = insertText(oldText, req.Line, req.Text); err != nil {
			return err
		}
	case OpRename:
		if err := p.requireExisting(); err != nil {
			return err
		}
		if _, err := os.Stat(req.NewPath); err == nil {
			return types.NewError(types.ErrorKindConflict, fmt.Sprintf(
				"edit: cannot rename to %s; it already exists", req.newDisplay()))
		} else if !os.IsNotExist(err) {
			return types.WrapError(types.ErrorKindInternal, "edit: stat "+req.newDisplay(), err)
		}
		p.newRaw = p.oldRaw
		return nil
	}

	p.newRaw = encode(newText, enc)
	if len(p.newRaw) > maxEditBytes {
		return types.NewError(types.ErrorKindValidation, fmt.Sprintf(
			"edit: result would be %d bytes, over the %d byte limit", len(p.newRaw), maxEditBytes))
	}
	p.diff = unifiedDiff(req.display(), oldText, newText, p.existed)
	return nil
}

// requireExisting rejects content operations against a missing file.
func (p *plan) requireExisting() error {
	if p.existed {
		return nil
	}
	return types.NewError(types.ErrorKindValidation,
		fmt.Sprintf("edit: %s does not exist", p.req.display()))
}

// commit writes the planned edit to disk.
func (e *Engine) commit(p *plan) error {
	if p.req.Op == OpRename {
		if err := os.MkdirAll(filepath.Dir(p.req.NewPath), 0o755); err != nil {
			return types.WrapError(types.ErrorKindInternal, "edit: create parent directory", err)
		}
		if err := os.Rename(p.req.Path, p.req.NewPath); err != nil {
			return types.WrapError(types.ErrorKindInternal, "edit: rename "+p.req.display(), err)
		}
		return nil
	}

	if !p.existed {
		if err := os.MkdirAll(filepath.Dir(p.req.Path), 0o755); err != nil {
			return types.WrapError(types.ErrorKindInternal, "edit: create parent directory", err)
		}
	}
	return writeAtomic(p.req.Path, p.newRaw, p.mode)
}

// result renders the plan as a Result.
func (p *plan) result(applied bool) *Result {
	res := &Result{
		Op:          p.req.Op,
		Path:        p.req.Path,
		NewPath:     p.req.NewPath,
		Applied:     applied,
		Created:     !p.existed,
		Occurrences: p.occurrences,
		NewHash:     hashBytes(p.newRaw),
		Bytes:       len(p.newRaw),
		Diff:        p.diff,
	}
	if p.existed {
		res.OldHash = hashBytes(p.oldRaw)
	}
	return res
}

// newDisplay names the rename destination in diffs and logs.
func (r Request) newDisplay() string {
	if r.NewDisplay != "" {
		return r.NewDisplay
	}
	return r.NewPath
}

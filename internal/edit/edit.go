// Package edit is Slick Code's file editing engine: the single
// subsystem through which files are modified. The Engine plans every
// edit in memory first — validating the request, detecting conflicts
// against the state the caller last saw, preserving the file's encoding
// and line endings, and rendering a unified diff — and only then
// commits it with an atomic write. Every applied edit yields a Rollback
// token that restores the prior state.
//
// The engine is provider-independent and knows nothing about tools or
// models; internal/tool/builtin adapts it into the tool framework, which
// remains the only execution path (permissions, dry-run, timeouts) for
// providers to request edits through.
package edit

import (
	"fmt"

	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// maxEditBytes caps the size of files the engine will edit and of the
// content it will write, keeping planned edits (which are held fully in
// memory, including rollback state) bounded.
const maxEditBytes = 4 << 20

// Op identifies one of the editing operations the engine supports.
type Op string

// Supported editing operations.
const (
	// OpCreate creates a new file; it fails if the file already exists.
	OpCreate Op = "create"

	// OpWrite sets a file's entire contents, creating it if absent.
	OpWrite Op = "write"

	// OpReplace replaces occurrences of a target string.
	OpReplace Op = "replace"

	// OpInsert inserts text at a 1-based line position.
	OpInsert Op = "insert"

	// OpDelete removes occurrences of a target string.
	OpDelete Op = "delete"

	// OpRename moves a file to a new path within the workspace.
	OpRename Op = "rename"
)

// Request describes one edit for the Engine to plan and apply. Paths
// are absolute; confining them to the working directory is the caller's
// responsibility (the tool layer resolves and confines every path
// before it reaches the engine).
type Request struct {
	// Op is the operation to perform.
	Op Op

	// Path is the absolute path of the file to edit.
	Path string

	// Display is the path as the user named it, used in diffs and
	// logs. Empty falls back to the base name of Path.
	Display string

	// NewPath is the absolute destination path for OpRename.
	NewPath string

	// NewDisplay is NewPath as the user named it.
	NewDisplay string

	// Content is the full file content for OpCreate and OpWrite.
	Content string

	// Target is the exact text to match for OpReplace and OpDelete.
	Target string

	// Replacement is the text substituted for Target by OpReplace.
	Replacement string

	// All applies OpReplace or OpDelete to every occurrence of Target.
	// When false, the target must occur exactly once.
	All bool

	// Line is the 1-based line position for OpInsert. Line N inserts
	// before the current line N; lineCount+1 appends.
	Line int

	// Text is the block inserted by OpInsert.
	Text string

	// BaseHash optionally guards against concurrent modification: when
	// set, the edit is refused with a conflict error unless the file's
	// current content hash matches. Preview results carry the hash to
	// pass here.
	BaseHash string
}

// Validate checks that the request is complete and consistent for its
// operation. It does not touch the filesystem.
func (r Request) Validate() error {
	if r.Path == "" {
		return types.NewError(types.ErrorKindValidation, "edit: path is required")
	}

	switch r.Op {
	case OpCreate, OpWrite:
		if len(r.Content) > maxEditBytes {
			return types.NewError(types.ErrorKindValidation, fmt.Sprintf(
				"edit: content is %d bytes, over the %d byte limit", len(r.Content), maxEditBytes))
		}
	case OpReplace, OpDelete:
		if r.Target == "" {
			return types.NewError(types.ErrorKindValidation, "edit: target text is required")
		}
	case OpInsert:
		if r.Line < 1 {
			return types.NewError(types.ErrorKindValidation,
				"edit: line must be 1 or greater")
		}
		if r.Text == "" {
			return types.NewError(types.ErrorKindValidation, "edit: text to insert is required")
		}
	case OpRename:
		if r.NewPath == "" {
			return types.NewError(types.ErrorKindValidation, "edit: new path is required")
		}
		if r.NewPath == r.Path {
			return types.NewError(types.ErrorKindValidation,
				"edit: new path is the same as the current path")
		}
	default:
		return types.NewError(types.ErrorKindValidation,
			fmt.Sprintf("edit: unknown operation %q", r.Op))
	}
	return nil
}

// display names the file in diffs and logs.
func (r Request) display() string {
	if r.Display != "" {
		return r.Display
	}
	return r.Path
}

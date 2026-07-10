package edit

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
)

// Result is the outcome of planning or applying one edit.
type Result struct {
	// Op is the operation that was performed or previewed.
	Op Op

	// Path is the absolute path of the edited file.
	Path string

	// NewPath is the destination path for OpRename.
	NewPath string

	// Applied reports whether the edit was committed to disk; false
	// means it was a preview and nothing changed.
	Applied bool

	// Created reports whether the edit brought the file into existence.
	Created bool

	// Occurrences is how many matches OpReplace or OpDelete acted on.
	Occurrences int

	// OldHash is the content hash of the file before the edit, empty
	// when the file did not exist. Passing it as Request.BaseHash on a
	// later edit detects concurrent modification.
	OldHash string

	// NewHash is the content hash of the file after the edit.
	NewHash string

	// Bytes is the size of the file content after the edit.
	Bytes int

	// Diff is the unified diff from the old content to the new, empty
	// when nothing changes (and for OpRename, which moves content
	// without modifying it).
	Diff string

	// Rollback restores the state before the edit; it is set only on
	// applied results.
	Rollback *Rollback
}

// Rollback captures everything needed to restore the state a file had
// before an applied edit, and is redeemed with Engine.Rollback. It
// refuses to restore over content that changed after the edit.
type Rollback struct {
	// Op is the operation being undone.
	Op Op

	// Path is the file's original absolute path.
	Path string

	// NewPath is where OpRename moved the file.
	NewPath string

	// Existed reports whether the file existed before the edit; when
	// false, rolling back removes it.
	Existed bool

	// Content is the raw bytes the file held before the edit.
	Content []byte

	// Mode is the file mode to restore.
	Mode fs.FileMode

	// NewHash is the content hash the edit produced, checked before
	// restoring so a rollback never clobbers later modifications.
	NewHash string
}

// hashBytes returns the hex-encoded SHA-256 of b, the engine's content
// hash used for conflict detection.
func hashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

package edit

import (
	"fmt"
	"strings"

	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// The patch engine: pure text transformations. Replace and delete
// operate on the raw text with exact matching, so every byte they don't
// touch — including line endings — passes through unchanged. Insert
// splices a block at a line boundary without re-joining the rest of the
// file, so existing line endings (even mixed ones) are preserved and
// the inserted block adopts the file's dominant ending.

// detectEOL returns the file's dominant line ending, defaulting to "\n"
// for files with no or evenly ambiguous endings.
func detectEOL(text string) string {
	crlf := strings.Count(text, "\r\n")
	lf := strings.Count(text, "\n")
	if crlf > 0 && crlf*2 >= lf {
		return "\r\n"
	}
	return "\n"
}

// countLines reports how many lines text has; a final line without a
// trailing newline still counts.
func countLines(text string) int {
	if text == "" {
		return 0
	}
	n := strings.Count(text, "\n")
	if !strings.HasSuffix(text, "\n") {
		n++
	}
	return n
}

// replaceText substitutes replacement for target. Without all, the
// target must occur exactly once, so an ambiguous match can never edit
// the wrong site.
func replaceText(text, target, replacement string, all bool) (string, int, error) {
	count := strings.Count(text, target)
	switch {
	case count == 0:
		return "", 0, types.NewError(types.ErrorKindValidation,
			"edit: target text not found in the file")
	case count > 1 && !all:
		return "", 0, types.NewError(types.ErrorKindValidation, fmt.Sprintf(
			"edit: target text occurs %d times; provide a longer, unique target or set all to change every occurrence", count))
	case all:
		return strings.ReplaceAll(text, target, replacement), count, nil
	default:
		return strings.Replace(text, target, replacement, 1), 1, nil
	}
}

// insertText splices block into text as new lines before the 1-based
// line position; lineCount+1 appends. The block's own newlines are
// normalized to the file's dominant ending, and one trailing newline on
// the block is ignored so "foo" and "foo\n" insert identically.
func insertText(text string, line int, block string) (string, error) {
	total := countLines(text)
	if line > total+1 {
		return "", types.NewError(types.ErrorKindValidation, fmt.Sprintf(
			"edit: line %d is out of range; the file has %d lines, so the insert position may be 1 to %d",
			line, total, total+1))
	}

	eol := detectEOL(text)
	block = strings.ReplaceAll(block, "\r\n", "\n")
	block = strings.TrimSuffix(block, "\n")
	block = strings.ReplaceAll(block, "\n", eol)

	if line == total+1 {
		switch {
		case text == "":
			return block + eol, nil
		case strings.HasSuffix(text, "\n"):
			return text + block + eol, nil
		default:
			// The file has no final newline; keep it that way.
			return text + eol + block, nil
		}
	}

	offset := 0
	for i := 1; i < line; i++ {
		offset += strings.IndexByte(text[offset:], '\n') + 1
	}
	return text[:offset] + block + eol + text[offset:], nil
}

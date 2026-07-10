package edit

import (
	"fmt"
	"strings"
)

const (
	// diffContext is how many unchanged lines surround each hunk.
	diffContext = 3

	// maxDiffCells caps the LCS table (rows × columns, after trimming
	// the common prefix and suffix). Beyond it the diff degrades to one
	// whole-file remove/add hunk instead of risking excessive memory.
	maxDiffCells = 4_000_000
)

// diffOp is one line of an edit script: kept (' '), removed ('-'), or
// added ('+').
type diffOp struct {
	kind byte
	line string
}

// unifiedDiff renders the change from oldText to newText as a unified
// diff named after the edited file. It returns "" when the texts are
// equal. oldExists selects the /dev/null header for newly created
// files. The diff is presentation for humans and models, not a patch
// source: carriage returns are stripped for display.
func unifiedDiff(name string, oldText, newText string, oldExists bool) string {
	if oldText == newText {
		return ""
	}
	a, b := splitLines(oldText), splitLines(newText)

	prefix := 0
	for prefix < len(a) && prefix < len(b) && a[prefix] == b[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(a)-prefix && suffix < len(b)-prefix &&
		a[len(a)-1-suffix] == b[len(b)-1-suffix] {
		suffix++
	}

	ops := make([]diffOp, 0, len(a)+len(b)-prefix-suffix)
	for _, line := range a[:prefix] {
		ops = append(ops, diffOp{' ', line})
	}
	ops = append(ops, lcsOps(a[prefix:len(a)-suffix], b[prefix:len(b)-suffix])...)
	for _, line := range a[len(a)-suffix:] {
		ops = append(ops, diffOp{' ', line})
	}

	oldName := "a/" + name
	if !oldExists {
		oldName = "/dev/null"
	}
	var out strings.Builder
	fmt.Fprintf(&out, "--- %s\n+++ %s\n", oldName, "b/"+name)
	writeHunks(&out, ops)
	return out.String()
}

// splitLines breaks text into display lines: no trailing newline
// artifacts, carriage returns stripped.
func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	text = strings.TrimSuffix(text, "\n")
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSuffix(line, "\r")
	}
	return lines
}

// lcsOps computes the line-level edit script between a and b using a
// longest-common-subsequence table, falling back to remove-all/add-all
// when the table would be too large.
func lcsOps(a, b []string) []diffOp {
	ops := make([]diffOp, 0, len(a)+len(b))
	if len(a)*len(b) > maxDiffCells {
		for _, line := range a {
			ops = append(ops, diffOp{'-', line})
		}
		for _, line := range b {
			ops = append(ops, diffOp{'+', line})
		}
		return ops
	}

	n, m := len(a), len(b)
	table := make([][]int32, n+1)
	for i := range table {
		table[i] = make([]int32, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				table[i][j] = table[i+1][j+1] + 1
			} else {
				table[i][j] = max(table[i+1][j], table[i][j+1])
			}
		}
	}

	i, j := 0, 0
	for i < n && j < m {
		switch {
		case a[i] == b[j]:
			ops = append(ops, diffOp{' ', a[i]})
			i++
			j++
		case table[i+1][j] >= table[i][j+1]:
			ops = append(ops, diffOp{'-', a[i]})
			i++
		default:
			ops = append(ops, diffOp{'+', b[j]})
			j++
		}
	}
	for ; i < n; i++ {
		ops = append(ops, diffOp{'-', a[i]})
	}
	for ; j < m; j++ {
		ops = append(ops, diffOp{'+', b[j]})
	}
	return ops
}

// writeHunks groups the edit script into unified-diff hunks with
// diffContext lines of context and writes them to out.
func writeHunks(out *strings.Builder, ops []diffOp) {
	// Line numbers each op starts at, so hunk headers can be computed
	// for any slice of the script.
	aNum := make([]int, len(ops)+1)
	bNum := make([]int, len(ops)+1)
	aPos, bPos := 1, 1
	for i, op := range ops {
		aNum[i], bNum[i] = aPos, bPos
		if op.kind != '+' {
			aPos++
		}
		if op.kind != '-' {
			bPos++
		}
	}
	aNum[len(ops)], bNum[len(ops)] = aPos, bPos

	for start := 0; start < len(ops); {
		// Find the next run of changes, expanded by context, and merge
		// runs whose context would overlap.
		first := start
		for first < len(ops) && ops[first].kind == ' ' {
			first++
		}
		if first == len(ops) {
			return
		}
		last := first
		for probe := first; probe < len(ops); probe++ {
			if ops[probe].kind != ' ' {
				last = probe
				continue
			}
			if probe-last > 2*diffContext {
				break
			}
		}

		lo := max(first-diffContext, 0)
		hi := min(last+diffContext+1, len(ops))

		aCount, bCount := 0, 0
		for _, op := range ops[lo:hi] {
			if op.kind != '+' {
				aCount++
			}
			if op.kind != '-' {
				bCount++
			}
		}
		aStart, bStart := aNum[lo], bNum[lo]
		if aCount == 0 {
			aStart--
		}
		if bCount == 0 {
			bStart--
		}

		fmt.Fprintf(out, "@@ -%d,%d +%d,%d @@\n", aStart, aCount, bStart, bCount)
		for _, op := range ops[lo:hi] {
			out.WriteByte(op.kind)
			out.WriteString(op.line)
			out.WriteByte('\n')
		}
		start = hi
	}
}

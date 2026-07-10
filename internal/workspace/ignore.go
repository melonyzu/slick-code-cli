package workspace

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// IgnorePattern is one parsed .gitignore pattern.
type IgnorePattern struct {
	Pattern       string
	Negated       bool
	DirectoryOnly bool
	Anchored      bool
}

// ParseIgnore parses .gitignore syntax without accessing the filesystem.
func ParseIgnore(r io.Reader) ([]IgnorePattern, error) {
	scanner := bufio.NewScanner(r)
	patterns := make([]IgnorePattern, 0)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSuffix(scanner.Text(), "\r")
		line = trimUnescapedTrailingSpaces(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		pattern := IgnorePattern{}
		if strings.HasPrefix(line, `\#`) || strings.HasPrefix(line, `\!`) {
			line = line[1:]
		} else if strings.HasPrefix(line, "!") {
			pattern.Negated = true
			line = line[1:]
		}
		if strings.HasPrefix(line, "/") {
			pattern.Anchored = true
			line = strings.TrimPrefix(line, "/")
		}
		if strings.HasSuffix(line, "/") && !strings.HasSuffix(line, `\/`) {
			pattern.DirectoryOnly = true
			line = strings.TrimSuffix(line, "/")
		}
		line = unescapePattern(line)
		if line == "" {
			return nil, fmt.Errorf("gitignore: empty pattern on line %d", lineNumber)
		}
		pattern.Pattern = filepath.ToSlash(line)
		patterns = append(patterns, pattern)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("gitignore: read: %w", err)
	}
	return patterns, nil
}

type ignoreRule struct {
	base    string
	pattern IgnorePattern
	re      *regexp.Regexp
}

// IgnoreMatcher applies root and nested .gitignore files in declaration
// order. Rules are scoped to the directory containing their file.
type IgnoreMatcher struct {
	root  string
	rules []ignoreRule
}

// NewIgnoreMatcher returns an empty matcher rooted at root.
func NewIgnoreMatcher(root string) *IgnoreMatcher {
	return &IgnoreMatcher{root: filepath.Clean(root)}
}

// AddFile parses and adds rules from a .gitignore file. A missing file is a
// no-op, which keeps directory walking straightforward.
func (m *IgnoreMatcher) AddFile(path string) error {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("gitignore: open %s: %w", path, err)
	}
	defer f.Close()

	patterns, err := ParseIgnore(f)
	if err != nil {
		return fmt.Errorf("gitignore: parse %s: %w", path, err)
	}
	base, err := filepath.Rel(m.root, filepath.Dir(path))
	if err != nil {
		return fmt.Errorf("gitignore: scope %s: %w", path, err)
	}
	base = cleanRelative(base)
	for _, pattern := range patterns {
		re, compileErr := compileIgnore(pattern)
		if compileErr != nil {
			return fmt.Errorf("gitignore: pattern %q in %s: %w", pattern.Pattern, path, compileErr)
		}
		m.rules = append(m.rules, ignoreRule{base: base, pattern: pattern, re: re})
	}
	return nil
}

// Ignored reports whether a root-relative path is excluded.
func (m *IgnoreMatcher) Ignored(path string, isDir bool) bool {
	rel := cleanRelative(path)
	if rel == "" {
		return false
	}
	for _, segment := range strings.Split(rel, "/") {
		if hardIgnoredDirectory(segment) {
			return true
		}
	}

	ignored := false
	for _, rule := range m.rules {
		scoped, ok := withinBase(rel, rule.base)
		if !ok || (rule.pattern.DirectoryOnly && !isDir && !strings.Contains(scoped, "/")) {
			continue
		}
		if rule.re.MatchString(scoped) {
			ignored = !rule.pattern.Negated
		}
	}
	return ignored
}

func compileIgnore(pattern IgnorePattern) (*regexp.Regexp, error) {
	glob := pattern.Pattern
	prefix := "^"
	if !pattern.Anchored && !strings.Contains(glob, "/") {
		prefix = `(?:^|/)`
	}
	body, err := globRegex(glob)
	if err != nil {
		return nil, err
	}
	return regexp.Compile(prefix + body + `(?:/.*)?$`)
}

func globRegex(glob string) (string, error) {
	var b strings.Builder
	for i := 0; i < len(glob); i++ {
		switch glob[i] {
		case '*':
			if i+1 < len(glob) && glob[i+1] == '*' {
				i++
				if i+1 < len(glob) && glob[i+1] == '/' {
					i++
					b.WriteString(`(?:.*/)?`)
				} else {
					b.WriteString(`.*`)
				}
			} else {
				b.WriteString(`[^/]*`)
			}
		case '?':
			b.WriteString(`[^/]`)
		case '[':
			end := strings.IndexByte(glob[i+1:], ']')
			if end < 0 {
				return "", fmt.Errorf("unclosed character class")
			}
			end += i + 1
			class := glob[i+1 : end]
			if strings.HasPrefix(class, "!") {
				class = "^" + class[1:]
			}
			b.WriteByte('[')
			b.WriteString(class)
			b.WriteByte(']')
			i = end
		default:
			b.WriteString(regexp.QuoteMeta(string(glob[i])))
		}
	}
	return b.String(), nil
}

func withinBase(path, base string) (string, bool) {
	if base == "" {
		return path, true
	}
	if path == base {
		return "", true
	}
	prefix := base + "/"
	if !strings.HasPrefix(path, prefix) {
		return "", false
	}
	return strings.TrimPrefix(path, prefix), true
}

func cleanRelative(path string) string {
	path = filepath.ToSlash(filepath.Clean(path))
	if path == "." {
		return ""
	}
	return strings.TrimPrefix(path, "./")
}

func trimUnescapedTrailingSpaces(line string) string {
	for strings.HasSuffix(line, " ") && !strings.HasSuffix(line, `\ `) {
		line = strings.TrimSuffix(line, " ")
	}
	return line
}

func unescapePattern(pattern string) string {
	replacer := strings.NewReplacer(`\ `, " ", `\#`, "#", `\!`, "!", `\\`, `\`)
	return replacer.Replace(pattern)
}

func hardIgnoredDirectory(name string) bool {
	switch name {
	case ".git", ".hg", ".svn", "node_modules", "vendor", ".venv", "venv",
		"__pycache__", "dist", "build", "target", "coverage", ".idea":
		return true
	default:
		return false
	}
}

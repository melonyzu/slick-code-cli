package workspace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	defaultMaxFileSize  = int64(1024 * 1024)
	defaultMaxTotalSize = int64(64 * 1024 * 1024)
	defaultMaxFiles     = 20_000
)

// File is a collected text file and its cache-validation metadata.
type File struct {
	Path     string      `json:"path"`
	Language string      `json:"language"`
	Size     int64       `json:"size"`
	ModTime  time.Time   `json:"mod_time"`
	Mode     fs.FileMode `json:"mode"`
	Hash     string      `json:"hash"`
	Content  string      `json:"content"`
}

// Collection is one ignore-aware workspace scan.
type Collection struct {
	Files     []File
	Changed   []string
	Removed   []string
	Reused    int
	Skipped   int
	Truncated bool
	totalSize int64
}

// CollectorParams configures a Collector. Zero limits use safe defaults.
type CollectorParams struct {
	MaxFileSize  int64
	MaxTotalSize int64
	MaxFiles     int
	Logger       *slog.Logger
}

// Collector walks and reads project files.
type Collector struct {
	maxFileSize  int64
	maxTotalSize int64
	maxFiles     int
	logger       *slog.Logger
}

// NewCollector returns an ignore-aware file collector.
func NewCollector(params CollectorParams) *Collector {
	if params.MaxFileSize <= 0 {
		params.MaxFileSize = defaultMaxFileSize
	}
	if params.MaxFiles <= 0 {
		params.MaxFiles = defaultMaxFiles
	}
	if params.MaxTotalSize <= 0 {
		params.MaxTotalSize = defaultMaxTotalSize
	}
	return &Collector{
		maxFileSize:  params.MaxFileSize,
		maxTotalSize: params.MaxTotalSize,
		maxFiles:     params.MaxFiles,
		logger:       loggerOrDiscard(params.Logger),
	}
}

// Collect walks project using nested .gitignore rules. Unchanged files are
// reused from previous when size, mode, and nanosecond modification time all
// match, avoiding unnecessary reads and hashing during incremental refreshes.
func (c *Collector) Collect(ctx context.Context, project Project, previous []File) (Collection, error) {
	previousByPath := make(map[string]File, len(previous))
	for _, file := range previous {
		previousByPath[file.Path] = file
	}

	matcher := NewIgnoreMatcher(project.Root)
	result := Collection{}
	seen := make(map[string]bool)
	if err := c.walk(ctx, project.Root, project.Root, matcher, previousByPath, seen, &result); err != nil {
		return Collection{}, err
	}
	for path := range previousByPath {
		if !seen[path] {
			result.Removed = append(result.Removed, path)
		}
	}
	sort.Slice(result.Files, func(i, j int) bool { return result.Files[i].Path < result.Files[j].Path })
	sort.Strings(result.Changed)
	sort.Strings(result.Removed)
	c.logger.Info("workspace files collected",
		"root", project.Root, "files", len(result.Files), "reused", result.Reused,
		"changed", len(result.Changed), "removed", len(result.Removed), "skipped", result.Skipped)
	return result, nil
}

func (c *Collector) walk(
	ctx context.Context,
	root, dir string,
	matcher *IgnoreMatcher,
	previous map[string]File,
	seen map[string]bool,
	result *Collection,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := matcher.AddFile(filepath.Join(dir, ".gitignore")); err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("workspace: read directory %s: %w", dir, err)
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		path := filepath.Join(dir, entry.Name())
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("workspace: relative path %s: %w", path, err)
		}
		rel = filepath.ToSlash(rel)
		if entry.Type()&os.ModeSymlink != 0 {
			result.Skipped++
			continue
		}
		if entry.IsDir() {
			if matcher.Ignored(rel, true) {
				result.Skipped++
				continue
			}
			if err := c.walk(ctx, root, path, matcher, previous, seen, result); err != nil {
				return err
			}
			continue
		}
		if matcher.Ignored(rel, false) || knownBinaryPath(rel) {
			result.Skipped++
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("workspace: stat %s: %w", path, err)
		}
		if !info.Mode().IsRegular() || info.Size() > c.maxFileSize {
			result.Skipped++
			continue
		}
		if len(result.Files) >= c.maxFiles {
			result.Truncated = true
			result.Skipped++
			continue
		}
		if result.totalSize+info.Size() > c.maxTotalSize {
			result.Truncated = true
			result.Skipped++
			continue
		}
		seen[rel] = true
		if cached, ok := previous[rel]; ok && unchanged(cached, info) {
			result.Files = append(result.Files, cached)
			result.totalSize += cached.Size
			result.Reused++
			continue
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("workspace: read %s: %w", path, err)
		}
		if binaryContent(data) {
			result.Skipped++
			continue
		}
		content := string(data)
		sum := sha256.Sum256(data)
		result.Files = append(result.Files, File{
			Path:     rel,
			Language: DetectLanguage(rel, content),
			Size:     info.Size(),
			ModTime:  info.ModTime(),
			Mode:     info.Mode(),
			Hash:     hex.EncodeToString(sum[:]),
			Content:  content,
		})
		result.totalSize += info.Size()
		result.Changed = append(result.Changed, rel)
	}
	return nil
}

func unchanged(file File, info fs.FileInfo) bool {
	return file.Size == info.Size() && file.Mode == info.Mode() && file.ModTime.Equal(info.ModTime())
}

func binaryContent(data []byte) bool {
	return strings.IndexByte(string(data), 0) >= 0 || !utf8.Valid(data)
}

func knownBinaryPath(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".ico", ".pdf", ".zip", ".gz",
		".tar", ".7z", ".rar", ".exe", ".dll", ".so", ".dylib", ".a", ".o", ".class",
		".jar", ".woff", ".woff2", ".ttf", ".otf", ".mp3", ".mp4", ".mov", ".avi",
		".sqlite", ".db", ".lockb":
		return true
	default:
		return false
	}
}

package projectcontext

import (
	stdcontext "context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	gitrepo "github.com/melonyzu/slick-code-cli/internal/git"
	"github.com/melonyzu/slick-code-cli/internal/workspace"
)

// Refresh describes an initial build or incremental workspace refresh.
type Refresh struct {
	Changed   []string
	Removed   []string
	Reused    int
	Skipped   int
	Truncated bool
	Duration  time.Duration
}

// ServiceParams holds constructed context-engine dependencies.
type ServiceParams struct {
	Project   workspace.Project
	Collector *workspace.Collector
	Builder   *Builder
	CacheFile string
	Logger    *slog.Logger
	// Repository supplies optional Git metadata during refresh.
	Repository *gitrepo.Manager
}

// Service owns the current project snapshot and its incremental cache.
type Service struct {
	mu         sync.RWMutex
	refreshMu  sync.Mutex
	project    workspace.Project
	collector  *workspace.Collector
	builder    *Builder
	cacheFile  string
	logger     *slog.Logger
	repository *gitrepo.Manager
	files      []workspace.File
	snapshot   Snapshot
}

type cacheData struct {
	Version int              `json:"version"`
	Root    string           `json:"root"`
	Files   []workspace.File `json:"files"`
}

const cacheVersion = 1

// NewService loads a compatible cache and returns a ready context service.
// Call Refresh before first use to validate cached metadata against disk.
func NewService(params ServiceParams) (*Service, error) {
	if params.Collector == nil || params.Builder == nil {
		return nil, fmt.Errorf("project context: collector and builder are required")
	}
	service := &Service{
		project: params.Project, collector: params.Collector, builder: params.Builder,
		cacheFile: params.CacheFile, logger: contextLogger(params.Logger), repository: params.Repository,
	}
	if err := service.load(); err != nil {
		service.logger.Warn("project context cache ignored", "path", params.CacheFile, "error", err)
	}
	return service, nil
}

func contextLogger(logger *slog.Logger) *slog.Logger {
	if logger != nil {
		return logger
	}
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// Refresh incrementally collects files, rebuilds the bounded snapshot, and
// atomically persists the validated file cache.
func (s *Service) Refresh(ctx stdcontext.Context) (Refresh, error) {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()

	started := time.Now()
	s.mu.RLock()
	previous := append([]workspace.File(nil), s.files...)
	s.mu.RUnlock()

	collection, err := s.collector.Collect(ctx, s.project, previous)
	if err != nil {
		return Refresh{}, err
	}
	var repositoryState *RepositoryState
	if s.repository != nil {
		status, statusErr := s.repository.Status(ctx)
		if statusErr != nil {
			s.logger.Warn("project context Git metadata unavailable", "error", statusErr)
		} else {
			repositoryState = &RepositoryState{
				Branch: status.Branch.Name, Head: status.Branch.Head, Detached: status.Branch.Detached,
				Clean: status.Clean, Changed: len(status.Changes),
			}
		}
	}
	snapshot := s.builder.BuildWithRepository(s.project, collection.Files, repositoryState)

	s.mu.Lock()
	s.files = collection.Files
	s.snapshot = snapshot
	s.mu.Unlock()

	if err := s.save(collection.Files); err != nil {
		return Refresh{}, err
	}
	refresh := Refresh{
		Changed: collection.Changed, Removed: collection.Removed, Reused: collection.Reused,
		Skipped: collection.Skipped, Truncated: collection.Truncated || snapshot.Truncated,
		Duration: time.Since(started),
	}
	s.logger.Info("project context refreshed",
		"root", s.project.Root, "changed", len(refresh.Changed), "removed", len(refresh.Removed),
		"reused", refresh.Reused, "tokens", snapshot.Estimated, "budget", snapshot.Budget,
		"duration", refresh.Duration)
	return refresh, nil
}

// Snapshot returns an immutable copy of the current bounded context.
func (s *Service) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snapshot := s.snapshot
	snapshot.Files = append([]FileInfo(nil), s.snapshot.Files...)
	return snapshot
}

// Project returns the discovered workspace metadata.
func (s *Service) Project() workspace.Project {
	return s.project
}

func (s *Service) load() error {
	if s.cacheFile == "" {
		return nil
	}
	data, err := os.ReadFile(s.cacheFile)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("project context: read cache: %w", err)
	}
	var cached cacheData
	if err := json.Unmarshal(data, &cached); err != nil {
		return fmt.Errorf("project context: decode cache: %w", err)
	}
	if cached.Version != cacheVersion || filepath.Clean(cached.Root) != filepath.Clean(s.project.Root) {
		return fmt.Errorf("project context: incompatible cache")
	}
	s.files = cached.Files
	return nil
}

func (s *Service) save(files []workspace.File) error {
	if s.cacheFile == "" {
		return nil
	}
	data, err := json.Marshal(cacheData{Version: cacheVersion, Root: s.project.Root, Files: files})
	if err != nil {
		return fmt.Errorf("project context: encode cache: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.cacheFile), 0o700); err != nil {
		return fmt.Errorf("project context: create cache directory: %w", err)
	}
	temp, err := os.CreateTemp(filepath.Dir(s.cacheFile), ".context-*.tmp")
	if err != nil {
		return fmt.Errorf("project context: create cache file: %w", err)
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return fmt.Errorf("project context: protect cache file: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return fmt.Errorf("project context: write cache file: %w", err)
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return fmt.Errorf("project context: sync cache file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("project context: close cache file: %w", err)
	}
	if err := os.Rename(tempName, s.cacheFile); err != nil {
		return fmt.Errorf("project context: replace cache file: %w", err)
	}
	return nil
}

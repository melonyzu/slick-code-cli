// Package storage resolves the on-disk locations Slick Code uses for
// configuration and cache data, following OS conventions.
package storage

import (
	"fmt"
	"os"
	"path/filepath"
)

// Paths resolves Slick Code's on-disk locations. It is constructed once
// during application bootstrap via Discover and injected into anything
// that needs it, rather than resolved ad hoc.
type Paths struct {
	configDir string
	cacheDir  string
}

// Discover resolves Slick Code's config directory, creating it if it does
// not already exist.
func Discover() (*Paths, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("storage: resolve user config directory: %w", err)
	}

	dir := filepath.Join(base, "slickcode")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("storage: create config directory: %w", err)
	}

	cacheBase, err := os.UserCacheDir()
	if err != nil {
		return nil, fmt.Errorf("storage: resolve user cache directory: %w", err)
	}
	cacheDir := filepath.Join(cacheBase, "slickcode")
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		return nil, fmt.Errorf("storage: create cache directory: %w", err)
	}

	return &Paths{configDir: dir, cacheDir: cacheDir}, nil
}

// ConfigDir returns the directory Slick Code stores its configuration in.
func (p *Paths) ConfigDir() string {
	return p.configDir
}

// ConfigFile returns the path to a named file within the config
// directory.
func (p *Paths) ConfigFile(name string) string {
	return filepath.Join(p.configDir, name)
}

// CacheDir returns the directory Slick Code stores rebuildable cache data in.
func (p *Paths) CacheDir() string {
	return p.cacheDir
}

// CacheFile returns the path to a named file within the cache directory.
func (p *Paths) CacheFile(name string) string {
	return filepath.Join(p.cacheDir, name)
}

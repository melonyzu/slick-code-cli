package config_test

import (
	"path/filepath"
	"testing"

	"github.com/melonyzu/slick-code-cli/internal/config"
	"github.com/melonyzu/slick-code-cli/pkg/types"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), config.FileName)

	if config.Exists(path) {
		t.Fatal("config must not exist before Save")
	}

	want := &config.Config{
		Provider: types.ProviderAnthropic,
		Model:    "claude-sonnet-5",
		LogLevel: types.LogLevelDebug,
	}
	if err := config.Save(path, want); err != nil {
		t.Fatal(err)
	}
	if !config.Exists(path) {
		t.Fatal("config must exist after Save")
	}

	got, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if *got != *want {
		t.Fatalf("round trip mismatch: got %+v, want %+v", got, want)
	}
	if err := got.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestEnvironmentOverridesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), config.FileName)
	if err := config.Save(path, &config.Config{
		Provider: types.ProviderAnthropic,
		Model:    "claude-sonnet-5",
		LogLevel: types.LogLevelInfo,
	}); err != nil {
		t.Fatal(err)
	}

	t.Setenv("SLICKCODE_MODEL", "claude-haiku-4-5")

	got, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Model != "claude-haiku-4-5" {
		t.Fatalf("environment did not override file: %+v", got)
	}
}

func TestLoadMissingFileUsesDefaults(t *testing.T) {
	got, err := config.Load(filepath.Join(t.TempDir(), config.FileName))
	if err != nil {
		t.Fatal(err)
	}
	if got.Provider != "" || got.LogLevel != types.LogLevelInfo {
		t.Fatalf("unexpected defaults: %+v", got)
	}
}

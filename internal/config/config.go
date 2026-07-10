// Package config loads, validates, and persists Slick Code's
// configuration. Values resolve from, in ascending order of precedence,
// built-in defaults, the config file, and environment variables.
// Credentials never appear here; they live in the auth subsystem's
// secure storage.
package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
	goyaml "go.yaml.in/yaml/v3"

	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// FileName is the name of Slick Code's configuration file.
const FileName = "config.yaml"

// envPrefix is stripped from, and the remainder lowercased for, environment
// variables that override configuration values, e.g. SLICKCODE_LOG_LEVEL
// becomes the "log_level" key.
const envPrefix = "SLICKCODE_"

// Config holds Slick Code's resolved configuration.
type Config struct {
	// Provider is the AI provider the assistant uses. Empty means the
	// first-run setup has not completed yet.
	Provider types.Provider `koanf:"provider"`

	// Model is the default model requests are sent to.
	Model string `koanf:"model"`

	// LogLevel controls the verbosity of diagnostic output.
	LogLevel types.LogLevel `koanf:"log_level"`
}

// defaults returns a Config populated with Slick Code's built-in defaults.
func defaults() Config {
	return Config{
		LogLevel: types.LogLevelInfo,
	}
}

// Exists reports whether a configuration file is present at path.
func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Load resolves Slick Code's configuration. path is the config file to
// read; a missing file at path is not an error, it simply leaves the
// defaults and environment overrides in place. Load does not validate the
// result — call Config.Validate for that.
func Load(path string) (*Config, error) {
	k := koanf.New(".")

	if err := k.Load(structs.Provider(defaults(), "koanf"), nil); err != nil {
		return nil, fmt.Errorf("config: load defaults: %w", err)
	}

	if _, err := os.Stat(path); err == nil {
		if err := k.Load(file.Provider(path), yaml.Parser()); err != nil {
			return nil, fmt.Errorf("config: load file %s: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("config: stat %s: %w", path, err)
	}

	if err := k.Load(env.Provider(envPrefix, ".", envKey), nil); err != nil {
		return nil, fmt.Errorf("config: load environment: %w", err)
	}

	var cfg Config
	if err := k.Unmarshal("", &cfg); err != nil {
		return nil, fmt.Errorf("config: unmarshal: %w", err)
	}

	return &cfg, nil
}

// Save writes cfg to path, readable only by the current user. Secrets
// are never part of Config, so the file never holds credentials.
func Save(path string, cfg *Config) error {
	payload, err := goyaml.Marshal(map[string]string{
		"provider":  cfg.Provider.String(),
		"model":     cfg.Model,
		"log_level": string(cfg.LogLevel),
	})
	if err != nil {
		return fmt.Errorf("config: encode: %w", err)
	}

	if err := os.WriteFile(path, payload, 0o600); err != nil {
		return fmt.Errorf("config: write %s: %w", path, err)
	}
	return nil
}

// Validate reports an error of kind types.ErrorKindInvalidConfig if the
// configuration holds unacceptable values.
func (c Config) Validate() error {
	if !c.LogLevel.Valid() {
		return types.NewError(types.ErrorKindInvalidConfig,
			fmt.Sprintf("invalid log_level %q (want debug, info, warn, or error)", c.LogLevel))
	}
	if c.Provider != "" && !c.Provider.Valid() {
		return types.NewError(types.ErrorKindInvalidConfig,
			fmt.Sprintf("invalid provider %q", c.Provider))
	}
	return nil
}

// envKey maps an environment variable name, such as SLICKCODE_LOG_LEVEL, to
// its configuration key, log_level.
func envKey(name string) string {
	trimmed := strings.TrimPrefix(name, envPrefix)
	return strings.ToLower(trimmed)
}

// Package config loads, validates, and saves Slick Code configuration.
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

// FileName is the default configuration file name.
const FileName = "config.yaml"

// Environment variables use the SLICKCODE_ prefix.
const envPrefix = "SLICKCODE_"

// Config holds the resolved application configuration.
type Config struct {
	// Provider is the selected AI provider.
	Provider types.Provider `koanf:"provider"`

	// Model is the default model.
	Model string `koanf:"model"`

	// LogLevel controls diagnostic logging.
	LogLevel types.LogLevel `koanf:"log_level"`
}

// defaults returns the built-in configuration.
func defaults() Config {
	return Config{
		LogLevel: types.LogLevelWarn,
	}
}

// Exists reports whether path exists.
func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Load loads configuration from defaults, the config file, and environment
// variables, in that order.
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

// Save writes cfg to path.
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

// Validate checks whether the configuration is valid.
func (c Config) Validate() error {
	if !c.LogLevel.Valid() {
		return types.NewError(
			types.ErrorKindInvalidConfig,
			fmt.Sprintf("invalid log_level %q (want debug, info, warn, or error)", c.LogLevel),
		)
	}

	if c.Provider != "" && !c.Provider.Valid() {
		return types.NewError(
			types.ErrorKindInvalidConfig,
			fmt.Sprintf("invalid provider %q", c.Provider),
		)
	}

	return nil
}

// envKey converts an environment variable name to its configuration key.
func envKey(name string) string {
	return strings.ToLower(strings.TrimPrefix(name, envPrefix))
}

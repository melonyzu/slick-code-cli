// Package logging provides the application's structured logger.
package logging

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// New returns a logger configured with the requested log level.
func New(level types.LogLevel) (*slog.Logger, error) {
	slogLevel, err := toSlogLevel(level)
	if err != nil {
		return nil, err
	}

	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slogLevel})
	return slog.New(handler), nil
}

func toSlogLevel(level types.LogLevel) (slog.Level, error) {
	switch level {
	case types.LogLevelDebug:
		return slog.LevelDebug, nil
	case types.LogLevelInfo:
		return slog.LevelInfo, nil
	case types.LogLevelWarn:
		return slog.LevelWarn, nil
	case types.LogLevelError:
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("logging: unknown level %q", level)
	}
}

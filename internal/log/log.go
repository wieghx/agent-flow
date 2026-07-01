// Package log provides structured logging for Agent Flow binaries and libraries.
package log

import (
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
)

var (
	mu      sync.RWMutex
	logger  = slog.Default()
	handler slog.Handler = slog.Default().Handler()
)

// Options configures the root structured logger.
type Options struct {
	JSON      bool
	Level     slog.Level
	AddSource bool
	Output    io.Writer
}

// Init configures the root logger. Safe to call multiple times; the latest call wins.
func Init(opts Options) {
	mu.Lock()
	defer mu.Unlock()

	level := opts.Level
	if level == 0 {
		level = slog.LevelInfo
	}
	out := opts.Output
	if out == nil {
		out = os.Stdout
	}
	handlerOpts := &slog.HandlerOptions{
		Level:     level,
		AddSource: opts.AddSource,
	}
	var h slog.Handler
	if opts.JSON {
		h = slog.NewJSONHandler(out, handlerOpts)
	} else {
		h = slog.NewTextHandler(out, handlerOpts)
	}
	handler = h
	logger = slog.New(h)
	slog.SetDefault(logger)
}

// Handler returns the root slog.Handler for bridging to logr-based libraries.
func Handler() slog.Handler {
	mu.RLock()
	defer mu.RUnlock()
	return handler
}

// InitFromEnv configures logging from environment variables:
//   - AGENTFLOW_LOG_FORMAT=json for JSON output (default: text)
//   - AGENTFLOW_LOG_LEVEL=debug|info|warn|error (default: info)
func InitFromEnv() {
	jsonFormat := strings.EqualFold(os.Getenv("AGENTFLOW_LOG_FORMAT"), "json")
	Init(Options{
		JSON:      jsonFormat,
		Level:     ParseLevel(os.Getenv("AGENTFLOW_LOG_LEVEL")),
		AddSource: true,
	})
}

// ParseLevel maps a level string to slog.Level. Unknown values default to info.
func ParseLevel(raw string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Default returns the configured root logger.
func Default() *slog.Logger {
	mu.RLock()
	defer mu.RUnlock()
	return logger
}

// Component returns a logger tagged with a stable component name.
func Component(name string) *slog.Logger {
	mu.RLock()
	defer mu.RUnlock()
	return logger.With("component", name)
}
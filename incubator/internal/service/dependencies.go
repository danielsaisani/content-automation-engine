package service

import (
	"content-automation-engine/internal/api"
	"content-automation-engine/internal/app/clock"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/vrischmann/envconfig"
)

// Base service dependencies that _every_ service will need
type ServiceDependencies struct {
	Clock  api.Clock
	Logger *slog.Logger
}

// LogConfig controls how the application logs. Read from LOG_FORMAT and LOG_LEVEL.
type LogConfig struct {
	// Format is "text" or "json".
	Format string `envconfig:"default=text"`
	// Level is "debug", "info", "warn" or "error".
	Level string `envconfig:"default=info"`
}

// LoadLogConfig reads the logging configuration from the environment.
func LoadLogConfig() (*LogConfig, error) {
	cfg := &LogConfig{}
	if err := envconfig.InitWithPrefix(cfg, "LOG"); err != nil {
		return nil, fmt.Errorf("loading log config: %w", err)
	}
	return cfg, nil
}

// NewLogger builds a logger that writes to w in the configured format and level.
func NewLogger(cfg *LogConfig, w io.Writer) (*slog.Logger, error) {
	var level slog.Level
	if err := level.UnmarshalText([]byte(cfg.Level)); err != nil {
		return nil, fmt.Errorf("invalid log level %q: %w", cfg.Level, err)
	}
	opts := &slog.HandlerOptions{Level: level}

	switch strings.ToLower(cfg.Format) {
	case "json":
		return slog.New(slog.NewJSONHandler(w, opts)), nil
	case "text", "":
		return slog.New(slog.NewTextHandler(w, opts)), nil
	default:
		return nil, fmt.Errorf("invalid log format %q: want text or json", cfg.Format)
	}
}

// Shared service dependencies
func NewServiceDependencies() *ServiceDependencies {
	return &ServiceDependencies{
		Clock:  clock.NewRealClock(),
		Logger: slog.New(slog.NewTextHandler(os.Stdout, nil)),
	}
}

// NewServiceDependenciesWithLogger is NewServiceDependencies with a logger the caller built.
func NewServiceDependenciesWithLogger(logger *slog.Logger) *ServiceDependencies {
	return &ServiceDependencies{
		Clock:  clock.NewRealClock(),
		Logger: logger,
	}
}

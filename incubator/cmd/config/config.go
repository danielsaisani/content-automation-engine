package config

import (
	"content-automation-engine/internal/clock"
	"log/slog"
	"os"
)

type Config struct {
	Clock  clock.Clock
	Logger *slog.Logger
}

func NewConfig() *Config {
	return &Config{
		Clock:  clock.NewRealClock(),
		Logger: slog.New(slog.NewTextHandler(os.Stdout, nil)),
	}
}

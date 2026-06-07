package app

import (
	"content-automation-engine/internal/api"
	"content-automation-engine/internal/app/clock"
	"log/slog"
	"os"
)

// Base service dependencies that _every_ service will need
type ServiceDependencies struct {
	Clock  api.Clock
	Logger *slog.Logger
}

// Shared service dependencies
func NewServiceDependencies() *ServiceDependencies {
	return &ServiceDependencies{
		Clock:  clock.NewRealClock(),
		Logger: slog.New(slog.NewTextHandler(os.Stdout, nil)),
	}
}

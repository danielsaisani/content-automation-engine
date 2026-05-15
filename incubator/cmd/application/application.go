package application

import (
	"content-automation-engine/internal/api"
	"content-automation-engine/internal/clock"
	"context"
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

type Application interface {
	Run(ctx context.Context) error
	Healthy(ctx context.Context) bool
}

type ContentAutomationEngineApplication struct{}

func (a *ContentAutomationEngineApplication) Run(ctx context.Context) error    { return nil }
func (a *ContentAutomationEngineApplication) Healthy(ctx context.Context) bool { return true }

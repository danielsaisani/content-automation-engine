package application

import (
	"content-automation-engine/internal/clock"
	"context"
	"log/slog"
	"os"
)

type ServiceDependencies struct {
	Clock  clock.Clock
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

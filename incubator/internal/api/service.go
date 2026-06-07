package api

import "context"

type Service interface {
	// Initiates the main loop of the service
	Run(ctx context.Context) error
	// Performs a health check on the service to the consumer
	Healthy(ctx context.Context) bool
	// Name retrieves the name of the service
	Name(ctx context.Context) string
}

type Services interface {
	RunServices(ctx context.Context) error
	GetServiceInstance(ctx context.Context, serviceName string) (Service, error)
}

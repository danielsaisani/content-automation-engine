package api

import "context"

type Service interface {
	// Initiates the main loop of the service
	Run(ctx context.Context) error
	// Performs a health check on the service to the consumer
	Healthy(ctx context.Context) bool
}

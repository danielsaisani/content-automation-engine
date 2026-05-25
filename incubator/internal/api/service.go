package api

import "context"

type Service interface {
	// Initiates the main loop of the service
	Run(ctx context.Context) error
	// Performs a health check on the service to the consumer
	Healthy(ctx context.Context) bool
	// Name retrieves the name of the service
	Name(ctx context.Context) ServiceName
}

type ServiceName string

// Returns the string representation of the service name
func (sn ServiceName) String() string { return string(sn) }

var (
	SchedulerService         ServiceName = "SchedulerService"
	CreatorService           ServiceName = "CreatorService"
	ContentGenerationService ServiceName = "ContentGenerationService"
	PosterService            ServiceName = "PosterService"
	NotifierService          ServiceName = "NotifierService"
	WebService               ServiceName = "WebService"
)

type Services interface {
	RunServices(ctx context.Context) error
	GetServiceInstance(ctx context.Context, serviceName ServiceName) (Service, error)
}

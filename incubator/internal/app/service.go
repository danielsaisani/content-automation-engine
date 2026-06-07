package app

import (
	"content-automation-engine/internal/api"
	"context"
	"fmt"
)

type Services struct {
	services []api.Service
}

func (s *Services) RunServices(ctx context.Context) error {
	for _, service := range s.services {
		if err := service.Run(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (s *Services) GetServiceInstance(ctx context.Context, serviceName string) (api.Service, error) {
	for _, service := range s.services {
		if service.Name(ctx) == serviceName {
			return service, nil
		}
	}
	return nil, fmt.Errorf("service %s not found", serviceName)
}

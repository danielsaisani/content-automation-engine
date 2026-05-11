// Package api contains all the contracts that bespoke service dependencies must implement and conform to in order to be usable by the services. Communication between components should be via these contracts so that components remain swappable and there is no unncessary tight coupling
package api

import "context"

type ObjectRepository interface {
	// Generic getter function to search objects by some configuration
	Get(interface{}) (interface{}, error)
	// Generic putter function to place an object in the repository
	Put(interface{}) (bool, error)
	// Healthy performs a health check on the repository
	Healthy(ctx context.Context) bool
}

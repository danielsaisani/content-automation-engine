// Package api contains all the contracts that bespoke service dependencies must implement and conform to in order to be usable by the services. Communication between components should be via these contracts so that components remain swappable and there is no unncessary tight coupling.
// Interactions between services themselves should _also_ be via these contracts so that the services are not dependent on each other, but rather on an abstraction of what the other services do. This is particularly useful when it comes to accessing resources that are maintained by a specific service.
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

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

package api

import (
	sharedAPI "content-automation-engine/internal/api"
	"context"
)

// ContentService is the contract other services use to talk to the content service.
type ContentService interface {
	sharedAPI.Service
	// GetStoryByID returns the story with the given ID, or ErrStoryNotFound.
	GetStoryByID(ctx context.Context, id string) (Story, error)
	// ClaimStory reserves the best available story for the caller. A claimed story is not
	// handed to anyone else until the claim expires, so concurrent callers (or several
	// instances of the service) never get the same story. It returns ErrNoStoryAvailable
	// when there is nothing to claim.
	ClaimStory(ctx context.Context, holder string) (Story, error)
}

package api

import (
	"context"
	"time"
)

// ClaimOptions describes which story a caller wants and how long it may hold it.
type ClaimOptions struct {
	// Holder names the caller taking the claim, for debugging.
	Holder string
	// Now is the current time; claims that expired before it are free again.
	Now time.Time
	// TTL is how long the claim lasts. If the holder never finishes (a crash, a failed
	// upload), the story becomes available again after this long.
	TTL time.Duration
	// ExcludeIDs are story IDs that must not be returned, e.g. stories already posted.
	ExcludeIDs []string
	// MaxAttempts skips stories that have already been claimed this many times, so a story
	// that always fails is not retried forever. Zero means no limit.
	MaxAttempts int
	// MinBodyChars and MaxBodyChars bound the story body's length. Zero means no bound.
	MinBodyChars int
	MaxBodyChars int
}

// StoryRepository stores the stories the content service owns.
type StoryRepository interface {
	// Save stores a story. If a story with the same Source and SourceID already exists it
	// is left alone and created is false.
	Save(ctx context.Context, story Story) (id string, created bool, err error)
	// GetByID returns the story with the given ID, or ErrStoryNotFound.
	GetByID(ctx context.Context, id string) (Story, error)
	// ClaimNext atomically picks one unclaimed story matching opts and claims it, so that
	// no two callers are ever handed the same story. It returns ErrNoStoryAvailable when
	// nothing matches.
	ClaimNext(ctx context.Context, opts ClaimOptions) (Story, error)
	// Healthy performs a health check on the repository
	Healthy(ctx context.Context) bool
}

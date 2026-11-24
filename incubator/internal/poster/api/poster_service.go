package api

import (
	sharedapi "content-automation-engine/internal/api"
	"context"
	"time"
)

// Poster defines the interface for posting content to social media platforms.
type Poster interface {
	Post(ctx context.Context, input interface{}) error
}

// Publication is a finished video ready to go out on a platform.
type Publication struct {
	// VideoPath is the absolute path of the video file.
	VideoPath string
	// Caption is the text posted alongside the video.
	Caption string
}

// Platform is one place videos can be published to (TikTok, a dry run, ...).
type Platform interface {
	Name() string
	Publish(ctx context.Context, publication Publication) error
}

// PostedStories answers which stories the poster service has already dealt with.
// It is the poster service's query API over the collection it owns.
type PostedStories interface {
	// PostedStoryIDs returns the IDs of every story that has been posted or is being posted.
	PostedStoryIDs(ctx context.Context) ([]string, error)
}

// PosterService is the contract other services use to talk to the poster service.
type PosterService interface {
	sharedapi.Service
	PostedStories
}

// PostRepository stores which stories the poster service has posted.
type PostRepository interface {
	// Reserve records that storyID is about to be posted. It returns false, without an
	// error, if the story is already recorded, which is what makes a duplicate post
	// impossible: only one caller can ever reserve a given story.
	Reserve(ctx context.Context, storyID string, platform string, at time.Time) (bool, error)
	// MarkPosted records that the reserved story was published.
	MarkPosted(ctx context.Context, storyID string, at time.Time) error
	// Release removes a reservation after a failed post so the story can be tried again.
	Release(ctx context.Context, storyID string) error
	// StoryIDs returns every recorded story ID, whether reserved or posted.
	StoryIDs(ctx context.Context) ([]string, error)
	// Healthy performs a health check on the repository
	Healthy(ctx context.Context) bool
}

package api

import "context"

// Poster defines the interface for posting content to social media platforms.
type Poster interface {
	Post(ctx context.Context, input interface{}) error
}

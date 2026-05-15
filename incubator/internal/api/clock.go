package api

import (
	"context"
	"time"
)

type Clock interface {
	// Returns the clock's current time in UTC
	Now(ctx context.Context) time.Time
}

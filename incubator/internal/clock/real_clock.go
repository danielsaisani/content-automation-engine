package clock

import (
	"context"
	"time"
)

// RealClock is a clock that returns the current time
// It is used to get the current time in the scheduler
type RealClock struct{}

func NewRealClock() *RealClock {
	return &RealClock{}
}

func (c RealClock) Now(ctx context.Context) time.Time {
	return time.Now()
}

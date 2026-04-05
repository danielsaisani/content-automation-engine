package scheduler

import "time"

type Clock interface {
	Now() time.Time
}

// RealClock is a clock that returns the current time
// It is used to get the current time in the scheduler
type RealClock struct{}

func NewRealClock() *RealClock {
	return &RealClock{}
}

func (c RealClock) Now() time.Time {
	return time.Now()
}

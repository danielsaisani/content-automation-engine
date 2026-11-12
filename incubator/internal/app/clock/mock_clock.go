package clock

import (
	"context"
	"time"
)

type MockClock struct {
	time time.Time
}

func NewMockClock() *MockClock {
	return &MockClock{}
}

func (c MockClock) SetTime(newTime time.Time) {
	c.time = newTime
}

func (c MockClock) Now(ctx context.Context) time.Time {
	return c.time
}

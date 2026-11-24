package clock

import (
	"context"
	"sync"
	"time"
)

// MockClock is a clock whose time is set by hand, for tests.
type MockClock struct {
	mu   sync.RWMutex
	time time.Time
}

func NewMockClock() *MockClock {
	return &MockClock{}
}

func (c *MockClock) SetTime(newTime time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.time = newTime
}

// Advance moves the clock forward by d.
func (c *MockClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.time = c.time.Add(d)
}

func (c *MockClock) Now(ctx context.Context) time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.time
}

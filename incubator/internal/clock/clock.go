package clock

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

type MockClock struct {
	time time.Time
}

func NewMockClock() *MockClock {
	return &MockClock{}
}

func (c MockClock) SetTime(newTime time.Time) {
	c.time = newTime
}

func (c MockClock) Now() time.Time {
	return c.time
}

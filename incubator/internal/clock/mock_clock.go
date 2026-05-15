package clock

import "time"

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

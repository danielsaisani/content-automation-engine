package events

import (
	"time"

	"github.com/google/uuid"
)

// Generic base struct that all events must be based on
type Event struct {
	ID          string
	TriggeredAt time.Time
}

func NewEvent() *Event {
	return &Event{
		ID:          uuid.New().String(),
		TriggeredAt: time.Now(),
	}
}

type Topic string

type TopicTriggered struct {
	Event
	Topic Topic
}

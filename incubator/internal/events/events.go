package events

import (
	"content-automation-engine/internal/clock"
	"time"

	"github.com/google/uuid"
)

// Generic base struct that all events must be based on
type Event struct {
	ID          string
	TriggeredAt time.Time
}

// NewEvent creates a new base event that handles the ID allocation and timestamp, this should be used across all services to create emitted events
func NewEvent(clock clock.Clock) *Event {
	return &Event{
		ID:          uuid.New().String(),
		TriggeredAt: clock.Now(),
	}
}

type Topic string

// TODO: figure out better name for this or better way to emit "scheduled" events
// Right now, we infer that since this is a topic event, it implies the scheduler has scheduled an upload (but these are distinct events)
type TopicTriggered struct {
	Event
	Topic Topic
}

type Story struct {
	Title string
	Body  string
}

type CreatorEvent struct {
	Event
	Story Story
}

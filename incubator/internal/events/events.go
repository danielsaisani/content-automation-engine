package events

import (
	"content-automation-engine/internal/api"
	"context"
	"time"

	"github.com/google/uuid"
)

// Generic base struct that all events must be based on
type Event struct {
	ID          string
	TriggeredAt time.Time
}

// NewEvent creates a new base event that handles the ID allocation and timestamp, this should be used across all services to create emitted events
func NewEvent(clock api.Clock) *Event {
	return &Event{
		ID:          uuid.New().String(),
		TriggeredAt: clock.Now(context.TODO()),
	}
}

type TopicPayload string

// TODO: figure out better name for this or better way to emit "scheduled" events
// Right now, we infer that since this is a topic event, it implies the scheduler has scheduled an upload (but these are distinct events)
type TopicTriggered struct {
	Event
	Topic TopicPayload
}

type StoryPayload struct {
	Title string
	Body  string
}

// Event to signify a story has been scraped and is ready to be used for the next step in the pipeline or by any downstream consumers
type StoryScraped struct {
	Event
	Story StoryPayload
}

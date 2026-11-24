// Package events holds the events that flow between services.
//
// Events only say that something HAS happened. They carry identifiers, not data: a consumer
// that needs the details asks the producing service for them through that service's API.
// Every event channel has exactly one producer and one consumer.
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

// TopicTriggered is emitted by the scheduler when it is time to produce a new post.
// TODO: figure out better name for this or better way to emit "scheduled" events
// Right now, we infer that since this is a topic event, it implies the scheduler has scheduled an upload (but these are distinct events)
type TopicTriggered struct {
	Event
	Topic TopicPayload
}

// StorySelected is emitted by the content service once it has claimed a story for the pipeline.
// Consumers fetch the story itself with ContentService.GetStoryByID.
type StorySelected struct {
	Event
	StoryID string
}

// VideoReady is emitted by the generator service once a video for a story exists.
// Consumers fetch the video details with GeneratorService.GetVideoByID.
type VideoReady struct {
	Event
	StoryID string
	VideoID string
}

// Publish sends an event on a channel, giving up if the context is cancelled first.
// It reports whether the event was sent.
func Publish[T any](ctx context.Context, ch chan<- T, event T) bool {
	select {
	case ch <- event:
		return true
	case <-ctx.Done():
		return false
	}
}

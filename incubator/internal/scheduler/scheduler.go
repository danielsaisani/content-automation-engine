package scheduler

import (
	"content-automation-engine/cmd/application"
	"content-automation-engine/internal/clock"
	"content-automation-engine/internal/events"
	"context"
	"log/slog"
	"time"
)

// CreatorService is the service responsible for creating and ideating new content to post, this service implements the `Service` interface and so can be treated as such

type RealScheduler struct {
	clock    clock.Clock
	interval time.Duration
	eventBus chan<- events.TopicTriggered
	logger   *slog.Logger
}

func NewRealScheduler(serviceDependencies *application.ServiceDependencies, eventBus chan<- events.TopicTriggered) *RealScheduler {
	return &RealScheduler{
		clock:    serviceDependencies.Clock,
		interval: time.Hour,
		logger:   serviceDependencies.Logger,
		eventBus: eventBus,
	}
}

type Topic string

func (topic *Topic) Valid() bool {
	switch *topic {
	case "misc":
		return true
	default:
		return false
	}
}

func (s *RealScheduler) Run(ctx context.Context) error {
	s.logger.Info("Starting scheduler..")

	// TODO: replace with actual clock that is injected
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		s.logger.Info("Scheduler ticked..")
		select {
		case <-ticker.C:
			s.logger.Info("Triggering event..")
			topic := Topic("misc")
			s.eventBus <- events.TopicTriggered{
				Event: *events.NewEvent(s.clock),
				// TODO: Replace with actual topic
				Topic: events.TopicPayload(topic),
			}
		case <-ctx.Done():
			s.logger.Info("Scheduler stopped..")
			return nil
		}
	}
}

func (s *RealScheduler) Healthy(ctx context.Context) bool { return true }

package scheduler

import (
	"content-automation-engine/cmd/config"
	"content-automation-engine/internal/clock"
	"content-automation-engine/internal/events"
	"context"
	"log/slog"
	"time"
)

type RealScheduler struct {
	clock    clock.Clock
	interval time.Duration
	eventBus chan<- events.TopicTriggered
	logger   *slog.Logger
}

func NewRealScheduler(cfg *config.Config, eventBus chan<- events.TopicTriggered) *RealScheduler {
	return &RealScheduler{
		clock:    cfg.Clock,
		interval: time.Hour,
		logger:   cfg.Logger,
		eventBus: eventBus,
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
			s.eventBus <- events.TopicTriggered{
				Event: *events.NewEvent(s.clock),
				// TODO: Replace with actual topic
				Topic: "misc",
			}
		case <-ctx.Done():
			s.logger.Info("Scheduler stopped..")
			return nil
		}
	}

}

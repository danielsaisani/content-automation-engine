package creator

import (
	"content-automation-engine/cmd/config"
	"content-automation-engine/internal/clock"
	"content-automation-engine/internal/events"
	"log/slog"
)

type CreatorService struct {
	clock             clock.Clock
	logger            *slog.Logger
	schedulerEventBus chan<- events.TopicTriggered
	creatorEventBus   chan<- events.Story
}

func NewCreatorService(cfg *config.Config, schedulerEventBus chan<- events.TopicTriggered, creatorEventBus chan<- events.Story) *CreatorService {
	return &CreatorService{
		clock:             cfg.Clock,
		logger:            cfg.Logger,
		schedulerEventBus: schedulerEventBus,
		creatorEventBus:   creatorEventBus,
	}
}

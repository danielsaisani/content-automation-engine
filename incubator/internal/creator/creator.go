package creator

import (
	"content-automation-engine/cmd/application"
	"content-automation-engine/internal/api"
	"content-automation-engine/internal/clock"
	"content-automation-engine/internal/events"
	"context"
	"log/slog"
)

// CreatorService is the service responsible for creating and ideating new content to post, this service implements the `Service` interface and so can be treated as such
type CreatorService struct {
	clock             clock.Clock
	logger            *slog.Logger
	schedulerEventBus <-chan events.TopicTriggered
	creatorEventBus   chan<- events.StoryScraped
	StoryRepository   api.ObjectRepository
	StoryGenerator    api.Scraper
}

func NewCreatorService(serviceDependencies *application.ServiceDependencies, schedulerEventBus <-chan events.TopicTriggered, creatorEventBus chan<- events.StoryScraped, storyRepository api.ObjectRepository, storyGenerator api.Scraper) *CreatorService {
	return &CreatorService{
		clock:             serviceDependencies.Clock,
		logger:            serviceDependencies.Logger,
		schedulerEventBus: schedulerEventBus,
		creatorEventBus:   creatorEventBus,
		StoryRepository:   storyRepository,
		StoryGenerator:    storyGenerator,
	}
}

// Run initiates the main loop of the service to listen to the necessary channels and emit events to the necessary channels
func (c *CreatorService) Run(ctx context.Context) error {
	c.logger.Info("Starting creator..")

	for {
		select {
		case <-c.schedulerEventBus:
			c.logger.Info("Scheduled story retrieved!")
			// Consume from scheduler bus
		case <-ctx.Done():
			c.logger.Info("Creator shutting down..")
			return nil
		}
	}
}

func (c *CreatorService) Healthy(ctx context.Context) bool {
	// The repository is the only dependency I know of.. maybe there's other checks to be done too
	return c.StoryRepository.Healthy(ctx)
}

type Story struct {
	Title string
	Body  string
}

package creator

import (
	"content-automation-engine/cmd/application"
	sharedapi "content-automation-engine/internal/api"
	creatorapi "content-automation-engine/internal/creator/api"
	"content-automation-engine/internal/events"
	"context"
	"fmt"
	"log/slog"
)

// CreatorService is the service responsible for creating and ideating new content to post, this service implements the `Service` interface and so can be treated as such
type CreatorService struct {
	clock             sharedapi.Clock
	logger            *slog.Logger
	schedulerEventBus <-chan events.TopicTriggered
	creatorEventBus   chan<- events.StoryScraped
	StoryRepository   creatorapi.StoryObjectRepository
	StoryGenerator    creatorapi.Scraper
}

func NewCreatorService(
	serviceDependencies *application.ServiceDependencies,
	schedulerEventBus <-chan events.TopicTriggered,
	creatorEventBus chan<- events.StoryScraped,
	storyRepository creatorapi.StoryObjectRepository,
	storyGenerator creatorapi.Scraper,
) *CreatorService {
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

	// Start Go Routine to handle new posts from story generator and insert them into the repository
	go func() {
		c.logger.Info("Starting up story listener")
		for {
			select {
			case <-ctx.Done():
				return
			case story := <-c.StoryGenerator.Posts():
				if story == nil {
					continue
				}
				// loose filtering of stories
				if !story.NSFW && story.Body.Populated() {
					_, err := c.StoryRepository.Put(story)
					if err != nil {
						c.logger.Error("failed to save story", "err", err)
						continue
					}
					c.logger.Info(fmt.Sprintf("Saved story to repository. %s", story.Title))
				}
			}
		}
	}()

	for {
		select {
		case <-c.schedulerEventBus:
			c.logger.Info("Scheduled story retrieved!")
			// retrieve story from Mongo

		case <-ctx.Done():
			c.logger.Info("Creator shutting down..")
			return nil
		}
	}
}

func (c *CreatorService) Healthy(ctx context.Context) bool {
	return c.StoryRepository.Healthy(ctx)
}

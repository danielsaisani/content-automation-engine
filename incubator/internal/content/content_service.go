// Package content is the service that collects stories and hands them to the rest of the pipeline.
package content

import (
	sharedapi "content-automation-engine/internal/api"
	contentapi "content-automation-engine/internal/content/api"
	"content-automation-engine/internal/events"
	posterapi "content-automation-engine/internal/poster/api"
	"content-automation-engine/internal/service"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/vrischmann/envconfig"
)

// Config is read from environment variables prefixed with CONTENT_.
type Config struct {
	// ClaimTTL is how long a selected story stays reserved for the pipeline.
	ClaimTTL time.Duration `envconfig:"default=30m"`
	// MaxAttempts is how many times a story may be selected before it is given up on.
	MaxAttempts int `envconfig:"default=3"`
	// MinBodyChars and MaxBodyChars bound the length of stories selected for the pipeline.
	MinBodyChars int `envconfig:"default=300"`
	MaxBodyChars int `envconfig:"default=2500"`
}

// LoadConfig reads the content service configuration from the environment.
func LoadConfig() (*Config, error) {
	cfg := &Config{}
	if err := envconfig.InitWithPrefix(cfg, "CONTENT"); err != nil {
		return nil, fmt.Errorf("loading content config: %w", err)
	}
	return cfg, nil
}

var _ contentapi.ContentService = (*ContentService)(nil)

// ContentService is the service responsible for creating and ideating new content to post, this service implements the `Service` interface and so can be treated as such
type ContentService struct {
	clock             sharedapi.Clock
	logger            *slog.Logger
	cfg               *Config
	schedulerEventBus <-chan events.TopicTriggered
	contentEventBus   chan<- events.StorySelected
	storyRepository   contentapi.StoryRepository
	scraper           contentapi.Scraper
	postedStories     posterapi.PostedStories
}

// NewContentService builds the content service.
//
// schedulerEventBus, contentEventBus and postedStories may be nil when the publishing pipeline
// is switched off; the service then only stores scraped stories.
func NewContentService(
	serviceDependencies *service.ServiceDependencies,
	cfg *Config,
	schedulerEventBus <-chan events.TopicTriggered,
	contentEventBus chan<- events.StorySelected,
	storyRepository contentapi.StoryRepository,
	scraper contentapi.Scraper,
	postedStories posterapi.PostedStories,
) *ContentService {
	return &ContentService{
		clock:             serviceDependencies.Clock,
		logger:            serviceDependencies.Logger.With("service", "content"),
		cfg:               cfg,
		schedulerEventBus: schedulerEventBus,
		contentEventBus:   contentEventBus,
		storyRepository:   storyRepository,
		scraper:           scraper,
		postedStories:     postedStories,
	}
}

// Run initiates the main loop of the service to listen to the necessary channels and emit events to the necessary channels
func (c *ContentService) Run(ctx context.Context) error {
	c.logger.Info("Starting content service")

	for {
		select {
		case scraped := <-c.scraper.Posts():
			c.ingest(ctx, scraped)

		// A nil scheduler bus never fires, which is what we want when the pipeline is off.
		case trigger := <-c.schedulerEventBus:
			c.selectStory(ctx, trigger)

		case <-ctx.Done():
			c.logger.Info("Content service stopped")
			return nil
		}
	}
}

// ingest stores a scraped post as a story if it is usable.
func (c *ContentService) ingest(ctx context.Context, scraped *contentapi.ScrapedContent) {
	if scraped == nil {
		return
	}
	// loose filtering of stories
	if scraped.NSFW || strings.TrimSpace(scraped.Body) == "" {
		c.logger.Debug("Skipping scraped post", "sourceID", scraped.SourceID, "nsfw", scraped.NSFW)
		return
	}

	id, created, err := c.storyRepository.Save(ctx, contentapi.Story{
		Source:    scraped.Source,
		SourceID:  scraped.SourceID,
		Subreddit: scraped.Subreddit,
		Title:     scraped.Title,
		Body:      scraped.Body,
		URL:       scraped.URL,
		CreatedAt: c.clock.Now(ctx),
	})
	if err != nil {
		c.logger.Error("Failed to save story", "sourceID", scraped.SourceID, "err", err)
		return
	}
	if !created {
		c.logger.Debug("Story already stored", "sourceID", scraped.SourceID)
		return
	}
	c.logger.Info("Saved story", "storyID", id, "subreddit", scraped.Subreddit, "title", scraped.Title)
}

// selectStory claims a story for the pipeline and announces it.
func (c *ContentService) selectStory(ctx context.Context, trigger events.TopicTriggered) {
	story, err := c.ClaimStory(ctx, "pipeline")
	if errors.Is(err, contentapi.ErrNoStoryAvailable) {
		c.logger.Warn("Scheduler fired but no story is available", "topic", trigger.Topic)
		return
	}
	if err != nil {
		c.logger.Error("Failed to claim a story", "err", err)
		return
	}

	c.logger.Info("Story selected", "storyID", story.ID, "title", story.Title)
	events.Publish(ctx, c.contentEventBus, events.StorySelected{
		Event:   *events.NewEvent(c.clock),
		StoryID: story.ID,
	})
}

// ClaimStory reserves the best available story that has not been posted yet.
func (c *ContentService) ClaimStory(ctx context.Context, holder string) (contentapi.Story, error) {
	var exclude []string
	if c.postedStories != nil {
		posted, err := c.postedStories.PostedStoryIDs(ctx)
		if err != nil {
			return contentapi.Story{}, fmt.Errorf("listing posted stories: %w", err)
		}
		exclude = posted
	}

	return c.storyRepository.ClaimNext(ctx, contentapi.ClaimOptions{
		Holder:       holder,
		Now:          c.clock.Now(ctx),
		TTL:          c.cfg.ClaimTTL,
		ExcludeIDs:   exclude,
		MaxAttempts:  c.cfg.MaxAttempts,
		MinBodyChars: c.cfg.MinBodyChars,
		MaxBodyChars: c.cfg.MaxBodyChars,
	})
}

func (c *ContentService) GetStoryByID(ctx context.Context, id string) (contentapi.Story, error) {
	return c.storyRepository.GetByID(ctx, id)
}

func (c *ContentService) Healthy(ctx context.Context) bool {
	return c.storyRepository.Healthy(ctx) && c.scraper.Healthy(ctx)
}

func (c *ContentService) Name(ctx context.Context) string {
	return "Content"
}

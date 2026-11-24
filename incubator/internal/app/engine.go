package app

import (
	"content-automation-engine/internal/api"
	"content-automation-engine/internal/content"
	contentapi "content-automation-engine/internal/content/api"
	"content-automation-engine/internal/events"
	"content-automation-engine/internal/generator"
	generatorapi "content-automation-engine/internal/generator/api"
	"content-automation-engine/internal/poster"
	posterapi "content-automation-engine/internal/poster/api"
	"content-automation-engine/internal/scheduler"
	"content-automation-engine/internal/service"
	"context"
	"errors"
	"fmt"

	"golang.org/x/sync/errgroup"
)

// eventBufferSize is how many events a channel holds before its producer has to wait.
const eventBufferSize = 10

// Pipeline holds everything the publishing half of the engine needs: scheduling a post,
// generating its video and publishing it. A nil *Pipeline means the engine only scrapes.
type Pipeline struct {
	SchedulerConfig *scheduler.Config
	GeneratorConfig *generator.Config
	PosterConfig    *poster.Config
	PostRepository  posterapi.PostRepository
	Narrator        generatorapi.Narrator
	Renderer        generatorapi.Renderer
	Platform        posterapi.Platform
}

// EngineParts are the pieces an Engine is assembled from. Everything that touches the
// outside world (databases, Reddit, text-to-speech, ffmpeg, TikTok) comes in through an
// interface here, which is what lets tests run the whole engine against fakes.
type EngineParts struct {
	Dependencies    *service.ServiceDependencies
	ContentConfig   *content.Config
	StoryRepository contentapi.StoryRepository
	Scraper         contentapi.Scraper
	// Pipeline is optional.
	Pipeline *Pipeline
}

// Engine is the set of services wired together:
//
//	Scraper ──posts──▶ Content ──▶ stories collection
//	Scheduler ─TopicTriggered─▶ Content ─StorySelected─▶ Generator ─VideoReady─▶ Poster
//
// Every channel has exactly one producer and one consumer.
type Engine struct {
	scraper  contentapi.Scraper
	services []api.Service
}

var _ api.Services = (*Engine)(nil)

// NewEngine wires the services together.
func NewEngine(parts EngineParts) (*Engine, error) {
	if parts.Dependencies == nil || parts.ContentConfig == nil || parts.StoryRepository == nil || parts.Scraper == nil {
		return nil, errors.New("engine: dependencies, content config, story repository and scraper are required")
	}
	engine := &Engine{scraper: parts.Scraper}

	if parts.Pipeline == nil {
		parts.Dependencies.Logger.Info("Publishing pipeline is disabled; only scraping stories")
		engine.services = []api.Service{
			content.NewContentService(parts.Dependencies, parts.ContentConfig, nil, nil, parts.StoryRepository, parts.Scraper, nil),
		}
		return engine, nil
	}

	p := parts.Pipeline
	topicCh := make(chan events.TopicTriggered, eventBufferSize)
	storyCh := make(chan events.StorySelected, eventBufferSize)
	videoCh := make(chan events.VideoReady, eventBufferSize)

	schedulerService, err := scheduler.NewRealSchedulerService(parts.Dependencies, topicCh, p.SchedulerConfig)
	if err != nil {
		return nil, fmt.Errorf("engine: %w", err)
	}
	contentService := content.NewContentService(parts.Dependencies, parts.ContentConfig, topicCh, storyCh,
		parts.StoryRepository, parts.Scraper, poster.NewLedger(p.PostRepository))
	generatorService := generator.NewGeneratorService(parts.Dependencies, p.GeneratorConfig, storyCh, videoCh,
		contentService, p.Narrator, p.Renderer)
	posterService := poster.NewPosterService(parts.Dependencies, p.PosterConfig, videoCh,
		generatorService, p.PostRepository, p.Platform)

	engine.services = []api.Service{contentService, schedulerService, generatorService, posterService}
	return engine, nil
}

// RunServices runs the scraper and every service concurrently and blocks until ctx is
// cancelled. If any of them fails, the rest are stopped and the failure is returned.
func (e *Engine) RunServices(ctx context.Context) error {
	group, ctx := errgroup.WithContext(ctx)

	group.Go(func() error {
		if err := e.scraper.Run(ctx); err != nil {
			return fmt.Errorf("scraper: %w", err)
		}
		return nil
	})
	for _, svc := range e.services {
		group.Go(func() error {
			if err := svc.Run(ctx); err != nil {
				return fmt.Errorf("%s service: %w", svc.Name(ctx), err)
			}
			return nil
		})
	}

	return group.Wait()
}

func (e *Engine) GetServiceInstance(ctx context.Context, serviceName string) (api.Service, error) {
	for _, svc := range e.services {
		if svc.Name(ctx) == serviceName {
			return svc, nil
		}
	}
	return nil, fmt.Errorf("service %s not found", serviceName)
}

// Health reports each service's health by name.
func (e *Engine) Health(ctx context.Context) map[string]bool {
	health := make(map[string]bool, len(e.services))
	for _, svc := range e.services {
		health[svc.Name(ctx)] = svc.Healthy(ctx)
	}
	return health
}

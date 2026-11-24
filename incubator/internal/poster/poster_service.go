// Package poster is the service that publishes generated videos and records what was posted.
package poster

import (
	sharedapi "content-automation-engine/internal/api"
	"content-automation-engine/internal/events"
	generatorapi "content-automation-engine/internal/generator/api"
	posterapi "content-automation-engine/internal/poster/api"
	"content-automation-engine/internal/service"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/vrischmann/envconfig"
)

// Config is read from environment variables prefixed with POSTER_.
type Config struct {
	// Platform is "dryrun" (log only, the default) or "tiktok".
	Platform string `envconfig:"default=dryrun"`
	// Hashtags are appended to every caption.
	Hashtags string `envconfig:"default=#reddit #redditstories #storytime"`
	// MaxCaptionChars caps the caption length.
	MaxCaptionChars int `envconfig:"default=150"`
}

// LoadConfig reads the poster service configuration from the environment.
func LoadConfig() (*Config, error) {
	cfg := &Config{}
	if err := envconfig.InitWithPrefix(cfg, "POSTER"); err != nil {
		return nil, fmt.Errorf("loading poster config: %w", err)
	}
	return cfg, nil
}

var _ posterapi.PostedStories = (*Ledger)(nil)

// Ledger is the poster service's answer to "has this story been posted?", backed by the
// collection the poster service owns. Other services use it through posterapi.PostedStories.
type Ledger struct {
	repository posterapi.PostRepository
}

func NewLedger(repository posterapi.PostRepository) *Ledger {
	return &Ledger{repository: repository}
}

func (l *Ledger) PostedStoryIDs(ctx context.Context) ([]string, error) {
	return l.repository.StoryIDs(ctx)
}

var _ posterapi.PosterService = (*PosterService)(nil)

// PosterService publishes each generated video once.
type PosterService struct {
	*Ledger
	clock             sharedapi.Clock
	logger            *slog.Logger
	cfg               *Config
	generatorEventBus <-chan events.VideoReady
	generator         generatorapi.GeneratorService
	repository        posterapi.PostRepository
	platform          posterapi.Platform
}

func NewPosterService(
	serviceDependencies *service.ServiceDependencies,
	cfg *Config,
	generatorEventBus <-chan events.VideoReady,
	generator generatorapi.GeneratorService,
	repository posterapi.PostRepository,
	platform posterapi.Platform,
) *PosterService {
	return &PosterService{
		Ledger:            NewLedger(repository),
		clock:             serviceDependencies.Clock,
		logger:            serviceDependencies.Logger.With("service", "poster", "platform", platform.Name()),
		cfg:               cfg,
		generatorEventBus: generatorEventBus,
		generator:         generator,
		repository:        repository,
		platform:          platform,
	}
}

func (p *PosterService) Run(ctx context.Context) error {
	p.logger.Info("Starting poster service")

	for {
		select {
		case ready := <-p.generatorEventBus:
			if err := p.post(ctx, ready); err != nil {
				p.logger.Error("Failed to post video", "storyID", ready.StoryID, "videoID", ready.VideoID, "err", err)
			}
			// Posted, skipped or failed, the video file is no longer needed.
			if err := p.generator.DiscardVideo(ctx, ready.VideoID); err != nil {
				p.logger.Warn("Failed to discard video", "videoID", ready.VideoID, "err", err)
			}

		case <-ctx.Done():
			p.logger.Info("Poster service stopped")
			return nil
		}
	}
}

func (p *PosterService) post(ctx context.Context, ready events.VideoReady) error {
	video, err := p.generator.GetVideoByID(ctx, ready.VideoID)
	if err != nil {
		return fmt.Errorf("fetching video: %w", err)
	}

	// Reserve before publishing: if anyone has already posted (or is posting) this story,
	// the reservation fails and we stop here instead of posting it twice.
	reserved, err := p.repository.Reserve(ctx, ready.StoryID, p.platform.Name(), p.clock.Now(ctx))
	if err != nil {
		return err
	}
	if !reserved {
		p.logger.Warn("Story was already posted, skipping", "storyID", ready.StoryID)
		return nil
	}

	publication := posterapi.Publication{VideoPath: video.Path, Caption: Caption(video.Title, p.cfg.Hashtags, p.cfg.MaxCaptionChars)}
	if err := p.platform.Publish(ctx, publication); err != nil {
		if releaseErr := p.repository.Release(ctx, ready.StoryID); releaseErr != nil {
			p.logger.Error("Failed to release story after a failed post", "storyID", ready.StoryID, "err", releaseErr)
		}
		return fmt.Errorf("publishing: %w", err)
	}

	if err := p.repository.MarkPosted(ctx, ready.StoryID, p.clock.Now(ctx)); err != nil {
		// The video is out. The reservation stays in place, so the story is still never reposted.
		return fmt.Errorf("video was published but recording it failed: %w", err)
	}

	p.logger.Info("Posted video", "storyID", ready.StoryID, "videoID", ready.VideoID)
	return nil
}

// Caption builds the text posted with a video: the title, shortened if needed so the
// hashtags always fit within maxChars.
func Caption(title string, hashtags string, maxChars int) string {
	title = strings.TrimSpace(title)
	hashtags = strings.TrimSpace(hashtags)
	if hashtags == "" {
		return truncateRunes(title, maxChars)
	}
	if title == "" {
		return truncateRunes(hashtags, maxChars)
	}

	room := maxChars - utf8.RuneCountInString(hashtags) - 1
	if maxChars <= 0 {
		return title + " " + hashtags
	}
	if room <= 0 {
		return truncateRunes(title, maxChars)
	}
	return truncateRunes(title, room) + " " + hashtags
}

func truncateRunes(text string, maxChars int) string {
	if maxChars <= 0 || utf8.RuneCountInString(text) <= maxChars {
		return text
	}
	if maxChars <= 1 {
		return string([]rune(text)[:maxChars])
	}
	return strings.TrimSpace(string([]rune(text)[:maxChars-1])) + "…"
}

func (p *PosterService) Healthy(ctx context.Context) bool {
	return p.repository.Healthy(ctx)
}

func (p *PosterService) Name(ctx context.Context) string {
	return "Poster"
}

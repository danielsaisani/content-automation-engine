// Package generator is the service that turns a selected story into a video.
package generator

import (
	sharedapi "content-automation-engine/internal/api"
	contentapi "content-automation-engine/internal/content/api"
	"content-automation-engine/internal/events"
	generatorapi "content-automation-engine/internal/generator/api"
	"content-automation-engine/internal/generator/captions"
	"content-automation-engine/internal/service"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/google/uuid"
	"github.com/vrischmann/envconfig"
)

// Config is read from environment variables prefixed with GENERATOR_.
type Config struct {
	// OutputDir is where videos are written. Defaults to a folder in the system temp directory.
	OutputDir string `envconfig:"optional"`
	// MaxNarrationChars caps how much of a story is narrated.
	MaxNarrationChars int `envconfig:"default=2500"`
}

// LoadConfig reads the generator service configuration from the environment.
func LoadConfig() (*Config, error) {
	cfg := &Config{}
	if err := envconfig.InitWithPrefix(cfg, "GENERATOR"); err != nil {
		return nil, fmt.Errorf("loading generator config: %w", err)
	}
	if cfg.OutputDir == "" {
		cfg.OutputDir = filepath.Join(os.TempDir(), "content-automation-engine", "videos")
	}
	return cfg, nil
}

var _ generatorapi.GeneratorService = (*GeneratorService)(nil)

// GeneratorService narrates a story and renders it as a video.
type GeneratorService struct {
	clock             sharedapi.Clock
	logger            *slog.Logger
	cfg               *Config
	contentEventBus   <-chan events.StorySelected
	generatorEventBus chan<- events.VideoReady
	content           contentapi.ContentService
	narrator          generatorapi.Narrator
	renderer          generatorapi.Renderer

	mu     sync.RWMutex
	videos map[string]generatorapi.Video
}

func NewGeneratorService(
	serviceDependencies *service.ServiceDependencies,
	cfg *Config,
	contentEventBus <-chan events.StorySelected,
	generatorEventBus chan<- events.VideoReady,
	content contentapi.ContentService,
	narrator generatorapi.Narrator,
	renderer generatorapi.Renderer,
) *GeneratorService {
	return &GeneratorService{
		clock:             serviceDependencies.Clock,
		logger:            serviceDependencies.Logger.With("service", "generator"),
		cfg:               cfg,
		contentEventBus:   contentEventBus,
		generatorEventBus: generatorEventBus,
		content:           content,
		narrator:          narrator,
		renderer:          renderer,
		videos:            make(map[string]generatorapi.Video),
	}
}

// Run generates one video at a time, in the order stories are selected.
func (g *GeneratorService) Run(ctx context.Context) error {
	g.logger.Info("Starting generator service", "outputDir", g.cfg.OutputDir)

	for {
		select {
		case selected := <-g.contentEventBus:
			video, err := g.generate(ctx, selected.StoryID)
			if err != nil {
				// The story's claim simply runs out, after which it can be selected again.
				g.logger.Error("Failed to generate video", "storyID", selected.StoryID, "err", err)
				continue
			}

			g.logger.Info("Video ready", "storyID", video.StoryID, "videoID", video.ID, "duration", video.Duration.String())
			events.Publish(ctx, g.generatorEventBus, events.VideoReady{
				Event:   *events.NewEvent(g.clock),
				StoryID: video.StoryID,
				VideoID: video.ID,
			})

		case <-ctx.Done():
			g.logger.Info("Generator service stopped")
			return nil
		}
	}
}

func (g *GeneratorService) generate(ctx context.Context, storyID string) (generatorapi.Video, error) {
	story, err := g.content.GetStoryByID(ctx, storyID)
	if err != nil {
		return generatorapi.Video{}, fmt.Errorf("fetching story: %w", err)
	}

	narration := Narration(story, g.cfg.MaxNarrationChars)
	if narration == "" {
		return generatorapi.Video{}, fmt.Errorf("story has no text to narrate")
	}

	id := uuid.New().String()
	dir := filepath.Join(g.cfg.OutputDir, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return generatorapi.Video{}, fmt.Errorf("creating video folder: %w", err)
	}

	audioPath := filepath.Join(dir, "narration.mp3")
	videoPath := filepath.Join(dir, "video.mp4")

	g.logger.Info("Narrating story", "storyID", storyID, "chars", len(narration))
	if err := g.narrator.Narrate(ctx, narration, audioPath); err != nil {
		_ = os.RemoveAll(dir)
		return generatorapi.Video{}, err
	}

	g.logger.Info("Rendering video", "storyID", storyID)
	duration, err := g.renderer.Render(ctx, generatorapi.RenderRequest{
		AudioPath:  audioPath,
		Narration:  narration,
		OutputPath: videoPath,
	})
	if err != nil {
		_ = os.RemoveAll(dir)
		return generatorapi.Video{}, err
	}

	video := generatorapi.Video{ID: id, StoryID: storyID, Title: captions.Clean(story.Title), Path: videoPath, Duration: duration}
	g.mu.Lock()
	g.videos[id] = video
	g.mu.Unlock()
	return video, nil
}

// Narration builds the text read aloud for a story: its title, then its body.
func Narration(story contentapi.Story, maxChars int) string {
	title := captions.Clean(story.Title)
	body := captions.Clean(story.Body)

	text := body
	if title != "" {
		text = title
		if last := title[len(title)-1]; last != '.' && last != '!' && last != '?' {
			text += "."
		}
		if body != "" {
			text += " " + body
		}
	}
	return captions.Truncate(text, maxChars)
}

func (g *GeneratorService) GetVideoByID(ctx context.Context, id string) (generatorapi.Video, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	video, ok := g.videos[id]
	if !ok {
		return generatorapi.Video{}, generatorapi.ErrVideoNotFound
	}
	return video, nil
}

func (g *GeneratorService) DiscardVideo(ctx context.Context, id string) error {
	g.mu.Lock()
	video, ok := g.videos[id]
	delete(g.videos, id)
	g.mu.Unlock()

	if !ok {
		return generatorapi.ErrVideoNotFound
	}
	if err := os.RemoveAll(filepath.Dir(video.Path)); err != nil {
		return fmt.Errorf("discarding video: %w", err)
	}
	return nil
}

func (g *GeneratorService) Healthy(ctx context.Context) bool { return true }

func (g *GeneratorService) Name(ctx context.Context) string { return "Generator" }

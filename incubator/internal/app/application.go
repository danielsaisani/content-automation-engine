package app

import (
	"content-automation-engine/internal/api"
	"content-automation-engine/internal/content"
	contentrepository "content-automation-engine/internal/content/repository"
	"content-automation-engine/internal/content/scraper"
	"content-automation-engine/internal/generator"
	"content-automation-engine/internal/generator/narrator"
	"content-automation-engine/internal/generator/renderer"
	"content-automation-engine/internal/mongodb"
	"content-automation-engine/internal/poster"
	posterapi "content-automation-engine/internal/poster/api"
	"content-automation-engine/internal/poster/platform"
	posterrepository "content-automation-engine/internal/poster/repository"
	"content-automation-engine/internal/poster/tiktok"
	"content-automation-engine/internal/scheduler"
	"content-automation-engine/internal/service"
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/vrischmann/envconfig"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"golang.org/x/sync/errgroup"
)

// Config is the application-level configuration, read from ENV, PIPELINE_ENABLED and HTTP_PORT.
type Config struct {
	// Env is "dev" for local development (plain mongodb:// URL); anything else is production.
	Env string `envconfig:"optional"`
	// PipelineEnabled switches on scheduling, video generation and posting. When false the
	// engine only scrapes stories into MongoDB.
	PipelineEnabled bool `envconfig:"default=false"`
	// HTTPPort is the port the health endpoint listens on.
	HTTPPort int `envconfig:"default=8000"`
}

// LoadConfig reads the application configuration from the environment.
func LoadConfig() (*Config, error) {
	cfg := &Config{}
	if err := envconfig.Init(cfg); err != nil {
		return nil, fmt.Errorf("loading application config: %w", err)
	}
	return cfg, nil
}

const startupTimeout = 30 * time.Second

var _ api.Application = (*ContentAutomationEngineApplication)(nil)

// ContentAutomationEngineApplication builds the real engine from the environment and runs it.
type ContentAutomationEngineApplication struct {
	logger      *slog.Logger
	cfg         *Config
	mongoClient *mongo.Client
}

func NewContentAutomationEngineApplication(logger *slog.Logger) *ContentAutomationEngineApplication {
	return &ContentAutomationEngineApplication{logger: logger, cfg: &Config{HTTPPort: 8000}}
}

// Start builds every service and runs the engine. It blocks until ctx is cancelled (a clean
// stop, returning nil) or something fails (returning the error).
func (a *ContentAutomationEngineApplication) Start(ctx context.Context) error {
	a.logger.Info("Starting engine")

	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	a.cfg = cfg
	a.logger.Info("Loaded configuration", "env", cfg.Env, "pipelineEnabled", cfg.PipelineEnabled, "httpPort", cfg.HTTPPort)

	parts, err := a.buildParts(ctx)
	if err != nil {
		return err
	}

	engine, err := NewEngine(*parts)
	if err != nil {
		return err
	}

	group, ctx := errgroup.WithContext(ctx)
	group.Go(func() error { return serveHealth(ctx, cfg.HTTPPort, engine) })
	group.Go(func() error { return engine.RunServices(ctx) })

	a.logger.Info("Engine started")
	err = group.Wait()
	a.logger.Info("Engine stopped")
	return err
}

// buildParts loads each module's configuration and builds the real implementations.
func (a *ContentAutomationEngineApplication) buildParts(ctx context.Context) (*EngineParts, error) {
	deps := service.NewServiceDependenciesWithLogger(a.logger)

	contentCfg, err := content.LoadConfig()
	if err != nil {
		return nil, err
	}
	scraperCfg, err := scraper.LoadRedditScraperConfig()
	if err != nil {
		return nil, err
	}
	mongoCfg, err := mongodb.LoadConfig()
	if err != nil {
		return nil, err
	}

	// Never log the connection URL: it contains the password.
	a.logger.Info("Connecting to MongoDB", "host", mongoCfg.Host, "database", mongoCfg.Database)
	connectCtx, cancel := context.WithTimeout(ctx, startupTimeout)
	defer cancel()
	client, err := mongodb.Connect(connectCtx, mongoCfg.ConnectionURL(a.cfg.Env))
	if err != nil {
		return nil, fmt.Errorf("connecting to mongodb at %s: %w", mongoCfg.Host, err)
	}
	a.mongoClient = client
	a.logger.Info("Connected to MongoDB")
	db := client.Database(mongoCfg.Database)

	storyRepository := contentrepository.NewMongoStoryRepository(db)
	if err := storyRepository.EnsureIndexes(connectCtx); err != nil {
		return nil, err
	}

	parts := &EngineParts{
		Dependencies:    deps,
		ContentConfig:   contentCfg,
		StoryRepository: storyRepository,
		Scraper:         scraper.NewRedditScraper(a.logger, scraperCfg),
	}
	if !a.cfg.PipelineEnabled {
		return parts, nil
	}

	pipeline, err := a.buildPipeline(connectCtx, db)
	if err != nil {
		return nil, err
	}
	parts.Pipeline = pipeline
	return parts, nil
}

func (a *ContentAutomationEngineApplication) buildPipeline(ctx context.Context, db *mongo.Database) (*Pipeline, error) {
	schedulerCfg, err := scheduler.LoadConfig()
	if err != nil {
		return nil, err
	}
	generatorCfg, err := generator.LoadConfig()
	if err != nil {
		return nil, err
	}
	kokoroCfg, err := narrator.LoadKokoroConfig()
	if err != nil {
		return nil, err
	}
	rendererCfg, err := renderer.LoadConfig()
	if err != nil {
		return nil, err
	}
	posterCfg, err := poster.LoadConfig()
	if err != nil {
		return nil, err
	}

	postRepository := posterrepository.NewMongoPostRepository(db)
	if err := postRepository.EnsureIndexes(ctx); err != nil {
		return nil, err
	}

	publishTo, err := a.buildPlatform(posterCfg)
	if err != nil {
		return nil, err
	}
	a.logger.Info("Publishing pipeline is enabled", "platform", publishTo.Name(), "narrator", kokoroCfg.URL)

	return &Pipeline{
		SchedulerConfig: schedulerCfg,
		GeneratorConfig: generatorCfg,
		PosterConfig:    posterCfg,
		PostRepository:  postRepository,
		Narrator:        narrator.NewKokoroNarrator(kokoroCfg),
		Renderer:        renderer.NewFFmpegRenderer(rendererCfg),
		Platform:        publishTo,
	}, nil
}

func (a *ContentAutomationEngineApplication) buildPlatform(cfg *poster.Config) (posterapi.Platform, error) {
	switch cfg.Platform {
	case "dryrun":
		return platform.NewDryRun(a.logger), nil
	case "tiktok":
		tiktokCfg, err := tiktok.LoadPlatformConfig()
		if err != nil {
			return nil, err
		}
		return tiktok.NewPlatform(tiktokCfg, tiktok.NewTiktokClient(a.logger))
	default:
		return nil, fmt.Errorf("unknown poster platform %q: want dryrun or tiktok", cfg.Platform)
	}
}

// Shutdown releases what Start acquired. Call it after Start has returned.
func (a *ContentAutomationEngineApplication) Shutdown(ctx context.Context) error {
	if a.mongoClient == nil {
		return nil
	}
	if err := a.mongoClient.Disconnect(ctx); err != nil {
		return fmt.Errorf("disconnecting from mongodb: %w", err)
	}
	a.mongoClient = nil
	return nil
}

func (a *ContentAutomationEngineApplication) Name(ctx context.Context) string {
	return "ContentAutomationEngineApplication"
}

func (a *ContentAutomationEngineApplication) Port(ctx context.Context) api.Port {
	return api.Port(a.cfg.HTTPPort)
}

package app

import (
	creator "content-automation-engine/internal/content"
	content "content-automation-engine/internal/content/repository"
	"content-automation-engine/internal/content/scraper"
	"content-automation-engine/internal/events"
	"content-automation-engine/internal/scheduler"
	"content-automation-engine/internal/service"
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

type ContentAutomationEngineApplication struct{}

func (a *ContentAutomationEngineApplication) Start(ctx context.Context) error {
	var mongoRepositoryConfig *content.MongoStoryRepositoryConfig

	mongoRepositoryConfig = content.NewMongoStoryRepositoryConfig(os.Getenv("MONGO_USERNAME"), os.Getenv("MONGO_PASSWORD"), os.Getenv("MONGO_HOST"), os.Getenv("MONGO_APP_NAME"), os.Getenv("ENV"))

	repo := content.NewMongoStoryRepository(mongoRepositoryConfig)
	if err := repo.InitialiseClient(ctx); err != nil {
		return fmt.Errorf("story repository: %w", err)
	}

	// TODO: this configuration should be loaded in from the environment and not in the binary..
	subreddits := []string{
		"worldnews", "news", "politics", "europe", "ukpolitics", "upliftingnews", "AskReddit", "AmItheAsshole", "relationship_advice", "confession", "unpopularopinion", "changemyview", "LifeProTips", "self",
		"memes", "dankmemes", "funny", "AITAH",
	}

	redditScraper := scraper.NewRedditScraper(subreddits)

	serviceDependencies := service.NewServiceDependencies()

	err := redditScraper.Run(context.Background())

	if err != nil {
		serviceDependencies.Logger.Info(fmt.Sprintf("Error when running Reddit scraper!: %s", err))
		return nil
	}

	serviceDependencies.Logger.Info("Starting engine..")

	serviceDependencies.Logger.Info("Creating story repository")

	topicCh := make(chan events.TopicTriggered, 10)
	storyCh := make(chan events.StoryScraped, 10)

	// TOOD: run all services from main thread one by one
	// var services []api.Service

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	schedulerConfig := scheduler.MustGetConfig()

	scheduler := scheduler.NewRealSchedulerService(serviceDependencies, topicCh, schedulerConfig)
	go scheduler.Run(ctx)

	creator := creator.NewCreatorService(serviceDependencies, topicCh, storyCh, repo, redditScraper)
	go creator.Run(ctx)

	serviceDependencies.Logger.Info("Engine started..")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	serviceDependencies.Logger.Info("Engine stopping..")

	return nil
}

func (a *ContentAutomationEngineApplication) Shutdown(ctx context.Context) error { return nil }

func (a *ContentAutomationEngineApplication) Healthy(ctx context.Context) bool { return true }

func (a *ContentAutomationEngineApplication) Name(ctx context.Context) string {
	return "ContentAutomationEngineApplication"
}
func (a *ContentAutomationEngineApplication) Port(ctx context.Context) int { return 8000 }

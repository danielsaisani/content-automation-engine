package main

import (
	"content-automation-engine/cmd/application"
	"content-automation-engine/cmd/config"
	"content-automation-engine/internal/creator"
	"content-automation-engine/internal/creator/reddit"
	"content-automation-engine/internal/events"
	"content-automation-engine/internal/observability/notifier"
	"content-automation-engine/internal/scheduler"
	"context"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	cfg := config.NewConfig()
	cfg.Load(context.Background())

	redditScraper := reddit.NewRedditScraper([]string{"all"})

	serviceDependencies := application.NewServiceDependencies()

	serviceDependencies.Logger.Info("Starting engine..")

	serviceDependencies.Logger.Info("Creating story repository")

	topicCh := make(chan events.TopicTriggered, 10)
	storyCh := make(chan events.StoryScraped, 10)

	// TOOD: run all services from main thread one by one
	// var services []api.Service

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	scheduler := scheduler.NewRealScheduler(serviceDependencies, topicCh)
	go scheduler.Run(ctx)

	notifier := notifier.NewNotifierService(serviceDependencies, topicCh)
	go notifier.Run(ctx)

	creator := creator.NewCreatorService(serviceDependencies, topicCh, storyCh, cfg.StoryRepository, redditScraper)
	go creator.Run(ctx)

	serviceDependencies.Logger.Info("Engine started..")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	serviceDependencies.Logger.Info("Engine stopping..")
}

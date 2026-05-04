package main

import (
	"content-automation-engine/cmd/config"
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

	cfg.Logger.Info("Starting engine..")

	topicCh := make(chan events.TopicTriggered, 10)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	scheduler := scheduler.NewRealScheduler(cfg, topicCh)
	go scheduler.Run(ctx)

	notifier := notifier.NewNotifierService(cfg, topicCh)
	go notifier.Run(ctx)

	cfg.Logger.Info("Engine started..")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	cfg.Logger.Info("Engine stopping..")
}

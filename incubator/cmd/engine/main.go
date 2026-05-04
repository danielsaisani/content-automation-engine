package main

import (
	"content-automation-engine/internal/clock"
	"content-automation-engine/internal/events"
	"content-automation-engine/internal/scheduler"
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	logger.Info("Starting engine..")

	topicCh := make(chan events.TopicTriggered, 10)

	clock := clock.NewRealClock()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	scheduler := scheduler.NewRealScheduler(clock, logger, topicCh)
	go scheduler.Run(ctx)

	logger.Info("Engine started..")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Engine stopping..")
}

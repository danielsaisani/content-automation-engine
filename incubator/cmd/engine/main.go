package main

import (
	"content-automation-engine/internal/app"
	"content-automation-engine/internal/service"
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	os.Exit(run())
}

// run returns the process exit code: 0 after a clean stop, 1 after any failure, so that a
// broken deployment shows up as a crashing pod with the reason in its logs.
func run() int {
	// A plain logger first, so that even a bad logging configuration gets reported.
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	logCfg, err := service.LoadLogConfig()
	if err != nil {
		logger.Error("Engine failed to start", "err", err)
		return 1
	}
	if logger, err = service.NewLogger(logCfg, os.Stdout); err != nil {
		slog.New(slog.NewTextHandler(os.Stdout, nil)).Error("Engine failed to start", "err", err)
		return 1
	}

	// Cancelled on Ctrl-C or when Kubernetes stops the pod.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	engine := app.NewContentAutomationEngineApplication(logger)
	runErr := engine.Start(ctx)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := engine.Shutdown(shutdownCtx); err != nil {
		logger.Error("Engine did not shut down cleanly", "err", err)
	}

	if runErr != nil {
		logger.Error("Engine failed", "err", runErr)
		return 1
	}
	return 0
}

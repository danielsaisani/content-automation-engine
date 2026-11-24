package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

// HealthReporter reports the health of each named component.
type HealthReporter interface {
	Health(ctx context.Context) map[string]bool
}

// NewHealthHandler serves GET /healthz: 200 when every service is healthy, 503 otherwise,
// with a JSON body listing each service. It is what Kubernetes probes should call.
func NewHealthHandler(reporter HealthReporter) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		services := reporter.Health(ctx)
		healthy := true
		for _, ok := range services {
			healthy = healthy && ok
		}

		w.Header().Set("Content-Type", "application/json")
		if !healthy {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"healthy": healthy, "services": services})
	})
	return mux
}

// serveHealth serves the health endpoint until ctx is cancelled.
func serveHealth(ctx context.Context, port int, reporter HealthReporter) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("health server: %w", err)
	}

	server := &http.Server{Handler: NewHealthHandler(reporter), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("health server: %w", err)
	}
	return nil
}

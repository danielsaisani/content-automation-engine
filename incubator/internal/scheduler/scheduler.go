package scheduler

import (
	"content-automation-engine/internal/api"
	"content-automation-engine/internal/events"
	"content-automation-engine/internal/service"
	"context"
	"log"
	"log/slog"
	"time"

	"github.com/vrischmann/envconfig"
)

type Config struct {
	interval string `envconfig:"INTERVAL"`
}

// MustGetConfig loads in the configuration from environment variables for the Scheduler Service.
// If the parsing fails, then the application will come to a halt.
func MustGetConfig() *Config {
	cfg := &Config{}
	if err := envconfig.InitWithPrefix(cfg, "SCHEDULER"); err != nil {
		log.Fatal("Error loading config:", err)
	}
	return cfg
}

type RealScheduler struct {
	clock    api.Clock
	interval time.Duration
	eventBus chan<- events.TopicTriggered
	logger   *slog.Logger
}

func NewRealScheduler(serviceDependencies *service.ServiceDependencies, eventBus chan<- events.TopicTriggered, cfg *Config) *RealScheduler {
	interval, err := time.ParseDuration(cfg.interval)
	if err != nil {
		log.Fatal("Error translating interval into time duration:", err)
	}
	return &RealScheduler{
		clock:    serviceDependencies.Clock,
		interval: interval,
		logger:   serviceDependencies.Logger,
		eventBus: eventBus,
	}
}

type Topic string

func (topic Topic) Valid() bool {
	switch topic {
	case "misc":
		return true
	default:
		return false
	}
}

func (s *RealScheduler) Run(ctx context.Context) error {
	s.logger.Info("Starting scheduler..")

	// TODO: replace with actual clock that is injected
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			topic := Topic("misc")
			s.eventBus <- events.TopicTriggered{
				Event: *events.NewEvent(s.clock),
				// TODO: Replace with actual topic
				Topic: events.TopicPayload(topic),
			}
		case <-ctx.Done():
			s.logger.Info("Scheduler stopped..")
			return nil
		}
	}
}

func (s *RealScheduler) Healthy(ctx context.Context) bool { return true }

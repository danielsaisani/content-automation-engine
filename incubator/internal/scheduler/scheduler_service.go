package scheduler

import (
	"content-automation-engine/internal/api"
	"content-automation-engine/internal/events"
	schedulerapi "content-automation-engine/internal/scheduler/api"
	"content-automation-engine/internal/service"
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"time"

	"github.com/vrischmann/envconfig"
)

// Config is read from environment variables prefixed with SCHEDULER_.
type Config struct {
	// Interval is how often a post is scheduled, e.g. "6h" or "30m".
	// Read from SCHEDULER_INTERVAL (prefix + field name). Don't add an `envconfig:"..."`
	// name tag here: a custom name replaces the whole key, so the prefix gets dropped.
	Interval time.Duration
	// RunOnStart also schedules a post as soon as the service starts, instead of waiting
	// a full interval for the first one.
	RunOnStart bool `envconfig:"default=false"`
}

// LoadConfig reads the scheduler configuration from the environment.
func LoadConfig() (*Config, error) {
	cfg := &Config{}
	if err := envconfig.InitWithPrefix(cfg, "SCHEDULER"); err != nil {
		return nil, fmt.Errorf("loading scheduler config: %w", err)
	}
	return cfg, nil
}

// MustGetConfig loads in the configuration from environment variables for the Scheduler Service.
// If the parsing fails, then the application will come to a halt.
func MustGetConfig() *Config {
	cfg, err := LoadConfig()
	if err != nil {
		log.Fatal("Error loading config:", err)
	}
	return cfg
}

var _ schedulerapi.Scheduler = (*RealSchedulerService)(nil)

type RealSchedulerService struct {
	clock      api.Clock
	interval   time.Duration
	runOnStart bool
	eventBus   chan<- events.TopicTriggered
	logger     *slog.Logger
}

func NewRealSchedulerService(serviceDependencies *service.ServiceDependencies, eventBus chan<- events.TopicTriggered, cfg *Config) (*RealSchedulerService, error) {
	if cfg.Interval <= 0 {
		return nil, errors.New("scheduler interval must be greater than zero")
	}
	return &RealSchedulerService{
		clock:      serviceDependencies.Clock,
		interval:   cfg.Interval,
		runOnStart: cfg.RunOnStart,
		logger:     serviceDependencies.Logger.With("service", "scheduler"),
		eventBus:   eventBus,
	}, nil
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

func (s *RealSchedulerService) Run(ctx context.Context) error {
	s.logger.Info("Starting scheduler", "interval", s.interval.String(), "runOnStart", s.runOnStart)

	if s.runOnStart {
		s.trigger(ctx)
	}

	// TODO: replace with actual clock that is injected
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.trigger(ctx)
		case <-ctx.Done():
			s.logger.Info("Scheduler stopped")
			return nil
		}
	}
}

func (s *RealSchedulerService) trigger(ctx context.Context) {
	// TODO: Replace with actual topic
	topic := Topic("misc")
	s.logger.Info("Scheduling a post", "topic", string(topic))
	events.Publish(ctx, s.eventBus, events.TopicTriggered{
		Event: *events.NewEvent(s.clock),
		Topic: events.TopicPayload(topic),
	})
}

func (s *RealSchedulerService) Healthy(ctx context.Context) bool { return true }

func (s *RealSchedulerService) Name(ctx context.Context) string { return "Scheduler" }

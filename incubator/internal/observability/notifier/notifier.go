package notifier

import (
	"content-automation-engine/cmd/config"
	"content-automation-engine/internal/clock"
	"content-automation-engine/internal/events"
	"context"
	"log/slog"
	"time"
)

type NotifierService struct {
	clock    clock.Clock
	interval time.Duration
	logger   *slog.Logger
	// Yes.. this is hacky, but I don't see an instant solution to polymorphic event consolidation
	topicChan chan events.TopicTriggered
}

type NotificationPayload struct {
	Message Message
}

func NewNotifierService(cfg *config.Config, topicChan chan events.TopicTriggered) *NotifierService {
	return &NotifierService{
		clock:     cfg.Clock,
		interval:  time.Minute,
		logger:    cfg.Logger,
		topicChan: topicChan,
	}
}

func (n *NotifierService) Run(ctx context.Context) error {
	n.logger.Info("Starting notifier service..")

	// TODO: replace with actual clock that is injected
	ticker := time.NewTicker(n.interval)
	defer ticker.Stop()

	for {
		n.logger.Info("Notifier ticked..")
		select {
		case <-ticker.C:
			// TODO: Check for all notifications across all event channels
		case <-ctx.Done():
			n.logger.Info("Notifier stopped..")
			return nil
		}
	}
}

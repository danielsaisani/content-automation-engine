package notifier

import (
	"content-automation-engine/cmd/application"
	"content-automation-engine/internal/api"
	"content-automation-engine/internal/events"
	"context"
	"log/slog"
	"time"
)

type NotifierService struct {
	clock    api.Clock
	interval time.Duration
	logger   *slog.Logger
	// Yes.. this is hacky, but I don't see an instant solution to polymorphic event consolidation
	topicChan <-chan events.TopicTriggered
}

type NotificationPayload struct {
	Message Message
}

func NewNotifierService(serviceDependencies *application.ServiceDependencies, topicChan chan events.TopicTriggered) *NotifierService {
	return &NotifierService{
		clock:     serviceDependencies.Clock,
		interval:  time.Minute,
		logger:    serviceDependencies.Logger,
		topicChan: topicChan,
	}
}

func (n *NotifierService) Run(ctx context.Context) error {
	n.logger.Info("Starting notifier service..")

	for {
		select {
		case <-n.topicChan:
			n.logger.Info("Notification worthy event consumed!")
			// TODO: Check for all notifications across all event channels
		case <-ctx.Done():
			n.logger.Info("Notifier stopped..")
			return nil
		}
	}
}

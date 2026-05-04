package notifier

import (
	"content-automation-engine/internal/clock"
	"content-automation-engine/internal/events"
	"context"
	"log/slog"
)

type NotifierService struct {
	clock     clock.Clock
	logger    *slog.Logger
	topicChan chan events.TopicTriggered
}

type NotificationPayload struct {
	Message Message
}

// Sink is a interface that represents a sink for notifications
type Sink interface {
	Notify(ctx context.Context, payload NotificationPayload) error
}

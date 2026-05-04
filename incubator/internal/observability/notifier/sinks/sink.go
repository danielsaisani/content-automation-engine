package notifier

import (
	"content-automation-engine/internal/observability/notifier"
	"context"
)

// Sink is a interface that represents a sink for notifications
type Sink interface {
	Notify(ctx context.Context, payload notifier.NotificationPayload) error
}

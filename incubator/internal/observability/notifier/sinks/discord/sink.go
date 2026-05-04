package discord

import (
	"content-automation-engine/internal/observability/notifier"
	"context"
)

type DiscordSink struct{}

func NewDiscordSink() *DiscordSink {
	return &DiscordSink{}
}

func (d *DiscordSink) Notify(ctx context.Context, payload notifier.NotificationPayload) error {
	// TODO: Implement Discord notification
	return nil
}

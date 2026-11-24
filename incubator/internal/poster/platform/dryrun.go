// Package platform holds Platform implementations that are not tied to a real social network.
package platform

import (
	"content-automation-engine/internal/poster/api"
	"context"
	"fmt"
	"log/slog"
	"os"
)

var _ api.Platform = (*DryRun)(nil)

// DryRun pretends to publish: it checks the video exists and logs what it would have posted.
// It lets the whole pipeline run without touching a real account.
type DryRun struct {
	logger *slog.Logger
}

func NewDryRun(logger *slog.Logger) *DryRun {
	return &DryRun{logger: logger}
}

func (d *DryRun) Name() string { return "dryrun" }

func (d *DryRun) Publish(ctx context.Context, publication api.Publication) error {
	info, err := os.Stat(publication.VideoPath)
	if err != nil {
		return fmt.Errorf("dry run: video is missing: %w", err)
	}
	d.logger.Info("Dry run: not publishing", "video", publication.VideoPath, "bytes", info.Size(), "caption", publication.Caption)
	return nil
}

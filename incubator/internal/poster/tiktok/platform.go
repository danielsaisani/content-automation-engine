package tiktok

import (
	posterapi "content-automation-engine/internal/poster/api"
	"context"
	"errors"
	"fmt"

	"github.com/vrischmann/envconfig"
)

// PlatformConfig is read from environment variables prefixed with POSTER_TIKTOK_.
type PlatformConfig struct {
	// SessionID and DatacenterID are the "sessionid" and "tt-target-idc" cookies of a logged-in
	// TikTok session. Log in once on a machine with a browser (TiktokClient.Login saves
	// them to tiktok_sessions/), then provide them here as secrets.
	SessionID    string
	DatacenterID string `envconfig:"optional"`
	// Visibility is 0 for public, 1 for private, 2 for friends. Private by default so test
	// uploads never reach followers by accident.
	Visibility   int  `envconfig:"default=1"`
	AllowComment bool `envconfig:"default=true"`
	AllowDuet    bool `envconfig:"default=false"`
	AllowStitch  bool `envconfig:"default=false"`
}

// LoadPlatformConfig reads the TikTok platform configuration from the environment.
func LoadPlatformConfig() (*PlatformConfig, error) {
	cfg := &PlatformConfig{}
	if err := envconfig.InitWithPrefix(cfg, "POSTER_TIKTOK"); err != nil {
		return nil, fmt.Errorf("loading tiktok config: %w", err)
	}
	return cfg, nil
}

var _ posterapi.Platform = (*Platform)(nil)

// Platform adapts a Poster that understands *TiktokPostInput to the poster service's
// Platform contract.
type Platform struct {
	cfg    *PlatformConfig
	poster posterapi.Poster
}

// NewPlatform builds the TikTok platform around a client, giving it the configured session cookies.
func NewPlatform(cfg *PlatformConfig, client *TiktokClient) (*Platform, error) {
	if cfg.SessionID == "" {
		return nil, errors.New("tiktok: a session id is required")
	}

	client.Config.Cookies = TiktokCookies{"sessionid": cfg.SessionID}
	if cfg.DatacenterID != "" {
		client.Config.Cookies["tt-target-idc"] = cfg.DatacenterID
	}
	return NewPlatformWithPoster(cfg, client), nil
}

// NewPlatformWithPoster builds the platform around any Poster. Intended for tests.
func NewPlatformWithPoster(cfg *PlatformConfig, poster posterapi.Poster) *Platform {
	return &Platform{cfg: cfg, poster: poster}
}

func (p *Platform) Name() string { return "tiktok" }

func (p *Platform) Publish(ctx context.Context, publication posterapi.Publication) error {
	return p.poster.Post(ctx, &TiktokPostInput{
		VideoPath:      publication.VideoPath,
		Title:          publication.Caption,
		AllowComment:   p.cfg.AllowComment,
		AllowDuet:      p.cfg.AllowDuet,
		AllowStitch:    p.cfg.AllowStitch,
		VisibilityType: p.cfg.Visibility,
	})
}

package tests

import (
	posterapi "content-automation-engine/internal/poster/api"
	"content-automation-engine/internal/poster/tiktok"
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakePoster records the input the platform hands to the TikTok client.
type fakePoster struct {
	inputs []interface{}
	err    error
}

func (f *fakePoster) Post(ctx context.Context, input interface{}) error {
	f.inputs = append(f.inputs, input)
	return f.err
}

func TestPlatformTranslatesAPublicationIntoATiktokPost(t *testing.T) {
	client := &fakePoster{}
	cfg := &tiktok.PlatformConfig{SessionID: "session", Visibility: 1, AllowComment: true}

	p := tiktok.NewPlatformWithPoster(cfg, client)
	err := p.Publish(context.Background(), posterapi.Publication{VideoPath: "/videos/v.mp4", Caption: "A caption #tag"})

	require.NoError(t, err)
	assert.Equal(t, "tiktok", p.Name())
	require.Len(t, client.inputs, 1)
	input, ok := client.inputs[0].(*tiktok.TiktokPostInput)
	require.True(t, ok, "the client is given the input type it expects")
	assert.Equal(t, "/videos/v.mp4", input.VideoPath)
	assert.Equal(t, "A caption #tag", input.Title)
	assert.Equal(t, 1, input.VisibilityType)
	assert.True(t, input.AllowComment)
	assert.False(t, input.AllowDuet)
	assert.Nil(t, input.ScheduleTime, "videos are posted immediately")
}

func TestPlatformReportsUploadFailures(t *testing.T) {
	client := &fakePoster{err: errors.New("upload failed")}

	err := tiktok.NewPlatformWithPoster(&tiktok.PlatformConfig{SessionID: "session"}, client).
		Publish(context.Background(), posterapi.Publication{VideoPath: "/videos/v.mp4"})

	require.ErrorContains(t, err, "upload failed")
}

func TestNewPlatformGivesTheClientItsSessionCookies(t *testing.T) {
	client := tiktok.NewTiktokClient(slog.New(slog.NewTextHandler(io.Discard, nil)))

	_, err := tiktok.NewPlatform(&tiktok.PlatformConfig{SessionID: "session-cookie", DatacenterID: "useast5"}, client)

	require.NoError(t, err)
	assert.Equal(t, "session-cookie", client.Config.Cookies.GetSessionID())
	assert.Equal(t, "useast5", client.Config.Cookies.GetDCID())
}

func TestNewPlatformRequiresASession(t *testing.T) {
	client := tiktok.NewTiktokClient(slog.New(slog.NewTextHandler(io.Discard, nil)))

	_, err := tiktok.NewPlatform(&tiktok.PlatformConfig{}, client)

	require.Error(t, err)
}

func TestLoadPlatformConfig(t *testing.T) {
	_, err := tiktok.LoadPlatformConfig()
	require.Error(t, err, "the session id is required")

	t.Setenv("POSTER_TIKTOK_SESSION_ID", "session-cookie")
	cfg, err := tiktok.LoadPlatformConfig()
	require.NoError(t, err)
	assert.Equal(t, "session-cookie", cfg.SessionID)
	assert.Equal(t, 1, cfg.Visibility, "uploads are private unless configured otherwise")
	assert.True(t, cfg.AllowComment)

	t.Setenv("POSTER_TIKTOK_DATACENTER_ID", "useast5")
	t.Setenv("POSTER_TIKTOK_VISIBILITY", "0")
	cfg, err = tiktok.LoadPlatformConfig()
	require.NoError(t, err)
	assert.Equal(t, "useast5", cfg.DatacenterID)
	assert.Equal(t, 0, cfg.Visibility)
}

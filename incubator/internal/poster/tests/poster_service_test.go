package tests

import (
	"content-automation-engine/internal/events"
	generatorapi "content-automation-engine/internal/generator/api"
	"content-automation-engine/internal/poster"
	posterapi "content-automation-engine/internal/poster/api"
	"content-automation-engine/internal/poster/platform"
	"content-automation-engine/internal/testutil"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var posterConfig = &poster.Config{Platform: "fake", Hashtags: "#reddit #storytime", MaxCaptionChars: 150}

type posterHarness struct {
	service    *poster.PosterService
	repository *testutil.PostRepository
	generator  *testutil.GeneratorService
	platform   *testutil.Platform
	ready      chan events.VideoReady
}

func newPosterHarness(t *testing.T) *posterHarness {
	t.Helper()
	deps, _ := testutil.NewDependencies()
	videoPath := filepath.Join(t.TempDir(), "video.mp4")
	require.NoError(t, os.WriteFile(videoPath, []byte("fake video"), 0o644))

	h := &posterHarness{
		repository: testutil.NewPostRepository(),
		generator: &testutil.GeneratorService{Videos: map[string]generatorapi.Video{
			"video-1": {ID: "video-1", StoryID: "story-1", Title: "TIFU by testing", Path: videoPath},
			"video-2": {ID: "video-2", StoryID: "story-1", Title: "TIFU by testing", Path: videoPath},
		}},
		platform: &testutil.Platform{},
		ready:    make(chan events.VideoReady, 10),
	}
	h.service = poster.NewPosterService(deps, posterConfig, h.ready, h.generator, h.repository, h.platform)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- h.service.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		require.NoError(t, <-done)
	})
	return h
}

// awaitHandled waits until the service has finished with n VideoReady events.
func (h *posterHarness) awaitHandled(t *testing.T, n int) {
	t.Helper()
	require.True(t, testutil.Eventually(2*time.Second, func() bool { return len(h.generator.Discarded()) == n }),
		"the service did not finish handling %d event(s)", n)
}

func TestVideoIsPublishedAndRecorded(t *testing.T) {
	h := newPosterHarness(t)

	h.ready <- events.VideoReady{StoryID: "story-1", VideoID: "video-1"}
	h.awaitHandled(t, 1)

	require.Len(t, h.platform.Publications(), 1)
	publication := h.platform.Publications()[0]
	assert.Equal(t, h.generator.Videos["video-1"].Path, publication.VideoPath)
	assert.Equal(t, "TIFU by testing #reddit #storytime", publication.Caption)
	assert.Equal(t, []bool{true}, h.platform.VideoExisted, "the video is still on disk while it is published")
	assert.Equal(t, "posted", h.repository.Status("story-1"))
	assert.Equal(t, []string{"video-1"}, h.generator.Discarded(), "the video is discarded once posted")

	ids, err := h.service.PostedStoryIDs(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"story-1"}, ids)
}

func TestAStoryIsNeverPublishedTwice(t *testing.T) {
	h := newPosterHarness(t)

	// Two videos for the same story, e.g. from two instances that both selected it.
	h.ready <- events.VideoReady{StoryID: "story-1", VideoID: "video-1"}
	h.ready <- events.VideoReady{StoryID: "story-1", VideoID: "video-2"}
	h.awaitHandled(t, 2)

	assert.Len(t, h.platform.Publications(), 1, "the second video for the same story is skipped")
	assert.Equal(t, []string{"video-1", "video-2"}, h.generator.Discarded(), "the skipped video is still cleaned up")
}

func TestAFailedPublishCanBeRetriedLater(t *testing.T) {
	h := newPosterHarness(t)
	h.platform.Err = errors.New("tiktok said no")

	h.ready <- events.VideoReady{StoryID: "story-1", VideoID: "video-1"}
	h.awaitHandled(t, 1)

	assert.Empty(t, h.repository.Status("story-1"), "the reservation is released so the story can be retried")
	ids, err := h.service.PostedStoryIDs(context.Background())
	require.NoError(t, err)
	assert.Empty(t, ids)

	h.platform.Err = nil
	h.ready <- events.VideoReady{StoryID: "story-1", VideoID: "video-2"}
	h.awaitHandled(t, 2)

	assert.Len(t, h.platform.Publications(), 1)
	assert.Equal(t, "posted", h.repository.Status("story-1"))
}

func TestNothingIsPublishedWhenTheStoryCannotBeReserved(t *testing.T) {
	h := newPosterHarness(t)
	h.repository.ReserveErr = errors.New("database is down")

	h.ready <- events.VideoReady{StoryID: "story-1", VideoID: "video-1"}
	h.awaitHandled(t, 1)

	assert.Empty(t, h.platform.Publications(), "publishing without a record could lead to a repost")
}

func TestAnUnknownVideoIsSkipped(t *testing.T) {
	h := newPosterHarness(t)

	h.ready <- events.VideoReady{StoryID: "story-9", VideoID: "missing"}
	h.ready <- events.VideoReady{StoryID: "story-1", VideoID: "video-1"}
	h.awaitHandled(t, 2)

	assert.Len(t, h.platform.Publications(), 1, "the service keeps going after a bad event")
	assert.Empty(t, h.repository.Status("story-9"))
}

func TestCaption(t *testing.T) {
	assert.Equal(t, "A title #a #b", poster.Caption("A title", "#a #b", 150))
	assert.Equal(t, "A title", poster.Caption("  A title ", "", 150))
	assert.Equal(t, "#a #b", poster.Caption("", "#a #b", 150))

	long := poster.Caption("This title is far too long to fit in the space that is available", "#reddit #storytime", 40)
	assert.Equal(t, 40, utf8.RuneCountInString(long))
	assert.Contains(t, long, "… #reddit #storytime", "the title is shortened, never the hashtags")
}

func TestLedgerListsWhatTheRepositoryHolds(t *testing.T) {
	repository := testutil.NewPostRepository()
	_, _ = repository.Reserve(context.Background(), "story-b", "fake", time.Now())
	_, _ = repository.Reserve(context.Background(), "story-a", "fake", time.Now())
	require.NoError(t, repository.MarkPosted(context.Background(), "story-a", time.Now()))

	ids, err := poster.NewLedger(repository).PostedStoryIDs(context.Background())

	require.NoError(t, err)
	// A story that is still being posted counts too, so it is not selected again meanwhile.
	assert.Equal(t, []string{"story-a", "story-b"}, ids)
}

func TestDryRunPlatformChecksTheVideoExists(t *testing.T) {
	dryRun := platform.NewDryRun(slog.New(slog.NewTextHandler(io.Discard, nil)))
	video := filepath.Join(t.TempDir(), "video.mp4")
	require.NoError(t, os.WriteFile(video, []byte("fake"), 0o644))

	assert.Equal(t, "dryrun", dryRun.Name())
	require.NoError(t, dryRun.Publish(context.Background(), posterapi.Publication{VideoPath: video, Caption: "hi"}))
	require.Error(t, dryRun.Publish(context.Background(), posterapi.Publication{VideoPath: video + ".missing"}))
}

func TestLoadPosterConfig(t *testing.T) {
	defaults, err := poster.LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, "dryrun", defaults.Platform, "nothing is posted for real unless asked for")
	assert.Equal(t, "#reddit #redditstories #storytime", defaults.Hashtags)

	t.Setenv("POSTER_PLATFORM", "tiktok")
	t.Setenv("POSTER_MAX_CAPTION_CHARS", "100")
	cfg, err := poster.LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, "tiktok", cfg.Platform)
	assert.Equal(t, 100, cfg.MaxCaptionChars)
}

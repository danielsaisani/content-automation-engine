//go:build integration

// Package integration runs the engine with as much of the real stack as can run offline:
// real MongoDB repositories, the real Kokoro HTTP client (against a local stand-in server),
// the real ffmpeg renderer and the dry-run platform. Only Reddit is faked.
//
// It needs MONGO_TEST_URI (a disposable server) and ffmpeg/ffprobe; see
// internal/mongodb/integration for how to start a disposable MongoDB.
package integration

import (
	"content-automation-engine/internal/app"
	"content-automation-engine/internal/content"
	contentapi "content-automation-engine/internal/content/api"
	contentrepository "content-automation-engine/internal/content/repository"
	"content-automation-engine/internal/generator"
	"content-automation-engine/internal/generator/narrator"
	"content-automation-engine/internal/generator/renderer"
	"content-automation-engine/internal/mongodb"
	"content-automation-engine/internal/poster"
	posterapi "content-automation-engine/internal/poster/api"
	"content-automation-engine/internal/poster/platform"
	posterrepository "content-automation-engine/internal/poster/repository"
	"content-automation-engine/internal/scheduler"
	"content-automation-engine/internal/testutil"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// inspectingPlatform wraps the dry-run platform and inspects each video before it is discarded.
type inspectingPlatform struct {
	posterapi.Platform
	mu     sync.Mutex
	probes []string
}

func (p *inspectingPlatform) Publish(ctx context.Context, publication posterapi.Publication) error {
	probe, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "stream=codec_type,width,height",
		"-of", "csv=p=0", publication.VideoPath).Output()
	if err != nil {
		return fmt.Errorf("the generated video is not readable: %w", err)
	}
	p.mu.Lock()
	p.probes = append(p.probes, string(probe))
	p.mu.Unlock()
	return p.Platform.Publish(ctx, publication)
}

func (p *inspectingPlatform) seen() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.probes...)
}

func TestPipelineWithRealMongoNarratorClientAndFFmpeg(t *testing.T) {
	uri := os.Getenv("MONGO_TEST_URI")
	if uri == "" {
		t.Skip("MONGO_TEST_URI is not set")
	}
	for _, tool := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not installed", tool)
		}
	}
	ctx := context.Background()

	// A disposable database.
	client, err := mongodb.Connect(ctx, uri)
	require.NoError(t, err)
	db := client.Database(fmt.Sprintf("engine_pipeline_test_%d", time.Now().UnixNano()))
	t.Cleanup(func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	})
	stories := contentrepository.NewMongoStoryRepository(db)
	require.NoError(t, stories.EnsureIndexes(ctx))
	posts := posterrepository.NewMongoPostRepository(db)
	require.NoError(t, posts.EnsureIndexes(ctx))

	// A stand-in for the Kokoro server that answers with real audio (a 2 second tone).
	tone := filepath.Join(t.TempDir(), "tone.mp3")
	out, err := exec.Command("ffmpeg", "-y", "-loglevel", "error", "-f", "lavfi", "-i", "sine=frequency=330:duration=2", tone).CombinedOutput()
	require.NoError(t, err, string(out))
	kokoro := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, tone)
	}))
	defer kokoro.Close()

	deps, _ := testutil.NewDependencies()
	scraper := testutil.NewScraper()
	publishTo := &inspectingPlatform{Platform: platform.NewDryRun(deps.Logger)}
	videoDir := t.TempDir()

	engine, err := app.NewEngine(app.EngineParts{
		Dependencies:    deps,
		ContentConfig:   &content.Config{ClaimTTL: 30 * time.Minute, MaxAttempts: 3, MinBodyChars: 20, MaxBodyChars: 2500},
		StoryRepository: stories,
		Scraper:         scraper,
		Pipeline: &app.Pipeline{
			SchedulerConfig: &scheduler.Config{Interval: 100 * time.Millisecond},
			GeneratorConfig: &generator.Config{OutputDir: videoDir, MaxNarrationChars: 2500},
			PosterConfig:    &poster.Config{Platform: "dryrun", Hashtags: "#storytime", MaxCaptionChars: 150},
			PostRepository:  posts,
			Narrator:        narrator.NewKokoroNarrator(&narrator.KokoroConfig{URL: kokoro.URL, Model: "kokoro", Voice: "af_heart", Timeout: 30 * time.Second}),
			Renderer: renderer.NewFFmpegRenderer(&renderer.Config{
				FfmpegPath: "ffmpeg", FfprobePath: "ffprobe", BackgroundColor: "0x14171f",
				CaptionFont: "DejaVu Sans", CaptionFontSize: 84, WordsPerCaption: 5,
			}),
			Platform: publishTo,
		},
	})
	require.NoError(t, err)

	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- engine.RunServices(runCtx) }()
	defer func() {
		cancel()
		require.NoError(t, <-done)
	}()

	scraper.Emit(&contentapi.ScrapedContent{
		Source: "reddit", SourceID: "t3_real", Subreddit: "tifu", Title: "TIFU by trusting the tests",
		Body: "It all started when the build went green. Then it stayed green, which was suspicious.",
	})
	// The same post arriving again must not create a second story or a second post.
	scraper.Emit(&contentapi.ScrapedContent{
		Source: "reddit", SourceID: "t3_real", Subreddit: "tifu", Title: "TIFU by trusting the tests",
		Body: "It all started when the build went green. Then it stayed green, which was suspicious.",
	})

	require.True(t, testutil.Eventually(60*time.Second, func() bool { return len(publishTo.seen()) == 1 }),
		"no video reached the platform")
	assert.Contains(t, publishTo.seen()[0], "video,1080,1920")
	assert.Contains(t, publishTo.seen()[0], "audio")

	// Give the scheduler several more runs: the story is recorded as posted, so nothing else goes out.
	time.Sleep(600 * time.Millisecond)
	assert.Len(t, publishTo.seen(), 1, "the story was posted more than once")

	ids, err := posts.StoryIDs(ctx)
	require.NoError(t, err)
	require.Len(t, ids, 1)
	story, err := stories.GetByID(ctx, ids[0])
	require.NoError(t, err)
	assert.Equal(t, "t3_real", story.SourceID)

	count, err := db.Collection(contentrepository.StoriesCollection).CountDocuments(ctx, map[string]any{})
	require.NoError(t, err)
	assert.EqualValues(t, 1, count, "the duplicate scrape was not stored")

	entries, err := os.ReadDir(videoDir)
	require.NoError(t, err)
	assert.Empty(t, entries, "video files were left behind")
}

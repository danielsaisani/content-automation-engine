package tests

import (
	"content-automation-engine/internal/app"
	"content-automation-engine/internal/content"
	contentapi "content-automation-engine/internal/content/api"
	"content-automation-engine/internal/generator"
	"content-automation-engine/internal/poster"
	"content-automation-engine/internal/scheduler"
	"content-automation-engine/internal/testutil"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// world is the whole engine wired to fakes: no database, Reddit, text-to-speech, ffmpeg or TikTok.
type world struct {
	engine   *app.Engine
	stories  *testutil.StoryRepository
	posts    *testutil.PostRepository
	scraper  *testutil.Scraper
	narrator *testutil.Narrator
	renderer *testutil.Renderer
	platform *testutil.Platform
	videoDir string
}

func newWorld(t *testing.T, interval time.Duration, pipeline bool) *world {
	t.Helper()
	deps, _ := testutil.NewDependencies()
	w := &world{
		stories:  testutil.NewStoryRepository(),
		posts:    testutil.NewPostRepository(),
		scraper:  testutil.NewScraper(),
		narrator: &testutil.Narrator{},
		renderer: &testutil.Renderer{},
		platform: &testutil.Platform{},
		videoDir: t.TempDir(),
	}

	parts := app.EngineParts{
		Dependencies:    deps,
		ContentConfig:   &content.Config{ClaimTTL: 30 * time.Minute, MaxAttempts: 3, MinBodyChars: 20, MaxBodyChars: 2000},
		StoryRepository: w.stories,
		Scraper:         w.scraper,
	}
	if pipeline {
		parts.Pipeline = &app.Pipeline{
			SchedulerConfig: &scheduler.Config{Interval: interval},
			GeneratorConfig: &generator.Config{OutputDir: w.videoDir, MaxNarrationChars: 2500},
			PosterConfig:    &poster.Config{Platform: "fake", Hashtags: "#storytime", MaxCaptionChars: 150},
			PostRepository:  w.posts,
			Narrator:        w.narrator,
			Renderer:        w.renderer,
			Platform:        w.platform,
		}
	}

	engine, err := app.NewEngine(parts)
	require.NoError(t, err)
	w.engine = engine
	return w
}

// run starts the engine and stops it when the test ends.
func (w *world) run(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.engine.RunServices(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			require.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Fatal("the engine did not stop")
		}
	})
}

func redditPost(id string, title string) *contentapi.ScrapedContent {
	return &contentapi.ScrapedContent{
		Source: "reddit", SourceID: id, Subreddit: "tifu", Title: title,
		Body: "This is the body of " + title + ", and it is long enough to be worth narrating.",
	}
}

func TestAScrapedStoryIsNarratedRenderedAndPostedExactlyOnce(t *testing.T) {
	w := newWorld(t, 25*time.Millisecond, true)
	w.run(t)

	w.scraper.Emit(redditPost("t3_one", "TIFU by shipping on a Friday"))

	require.True(t, testutil.Eventually(3*time.Second, func() bool { return len(w.platform.Publications()) == 1 }),
		"the story never reached the platform")

	// Scraped and stored by the content service.
	require.Len(t, w.stories.Stories(), 1)
	story := w.stories.Stories()[0]

	// Narrated and rendered by the generator.
	require.Len(t, w.narrator.Texts(), 1)
	assert.True(t, strings.HasPrefix(w.narrator.Texts()[0], "TIFU by shipping on a Friday. This is the body"))
	require.Len(t, w.renderer.Requests(), 1)

	// Published and recorded by the poster.
	publication := w.platform.Publications()[0]
	assert.Equal(t, "TIFU by shipping on a Friday #storytime", publication.Caption)
	assert.Equal(t, []bool{true}, w.platform.VideoExisted)
	require.True(t, testutil.Eventually(time.Second, func() bool { return w.posts.Status(story.ID) == "posted" }))

	// The scheduler keeps firing, but the only story is already posted, so nothing else goes out.
	time.Sleep(150 * time.Millisecond)
	assert.Len(t, w.platform.Publications(), 1, "the story was posted more than once")
	assert.Len(t, w.narrator.Texts(), 1, "no video is generated for a story that is already posted")

	// The generated files are cleaned up after posting.
	require.True(t, testutil.Eventually(time.Second, func() bool {
		entries, err := os.ReadDir(w.videoDir)
		return err == nil && len(entries) == 0
	}), "video files were left behind")
}

func TestEveryStoryIsPostedOnceAcrossManyScheduledRuns(t *testing.T) {
	w := newWorld(t, 10*time.Millisecond, true)
	w.run(t)

	titles := []string{"First story", "Second story", "Third story"}
	for i, title := range titles {
		w.scraper.Emit(redditPost("t3_"+string(rune('a'+i)), title))
	}

	require.True(t, testutil.Eventually(5*time.Second, func() bool { return len(w.platform.Publications()) == len(titles) }))
	time.Sleep(100 * time.Millisecond)

	posted := make(map[string]int)
	for _, publication := range w.platform.Publications() {
		posted[publication.Caption]++
	}
	assert.Len(t, posted, len(titles))
	for caption, count := range posted {
		assert.Equal(t, 1, count, "%q was posted %d times", caption, count)
	}
}

func TestAFailedUploadLeavesTheStoryUnposted(t *testing.T) {
	w := newWorld(t, 25*time.Millisecond, true)
	w.platform.Err = assert.AnError
	w.run(t)

	w.scraper.Emit(redditPost("t3_retry", "A story TikTok rejects at first"))
	require.True(t, testutil.Eventually(3*time.Second, func() bool { return len(w.renderer.Requests()) >= 1 }))
	time.Sleep(50 * time.Millisecond)

	assert.Empty(t, w.platform.Publications())
	story := w.stories.Stories()[0]
	assert.Empty(t, w.posts.Status(story.ID), "a failed upload leaves no record, so the story is not treated as posted")
}

func TestScrapeOnlyModeStoresStoriesAndPublishesNothing(t *testing.T) {
	w := newWorld(t, 10*time.Millisecond, false)
	w.run(t)

	w.scraper.Emit(redditPost("t3_one", "A stored story"))
	w.scraper.Emit(redditPost("t3_two", "Another stored story"))

	require.True(t, testutil.Eventually(2*time.Second, func() bool { return len(w.stories.Stories()) == 2 }))
	time.Sleep(60 * time.Millisecond)
	assert.Empty(t, w.narrator.Texts())
	assert.Empty(t, w.platform.Publications())

	_, err := w.engine.GetServiceInstance(context.Background(), "Poster")
	require.Error(t, err, "the poster does not exist in scrape-only mode")
	_, err = w.engine.GetServiceInstance(context.Background(), "Content")
	require.NoError(t, err)
}

func TestEngineExposesItsServicesByName(t *testing.T) {
	w := newWorld(t, time.Hour, true)

	for _, name := range []string{"Content", "Scheduler", "Generator", "Poster"} {
		svc, err := w.engine.GetServiceInstance(context.Background(), name)
		require.NoError(t, err, name)
		assert.Equal(t, name, svc.Name(context.Background()))
	}
	_, err := w.engine.GetServiceInstance(context.Background(), "Nope")
	require.Error(t, err)
}

func TestNewEngineRequiresItsCoreParts(t *testing.T) {
	_, err := app.NewEngine(app.EngineParts{})

	require.Error(t, err)
}

func TestNewEngineRejectsABadSchedule(t *testing.T) {
	deps, _ := testutil.NewDependencies()

	_, err := app.NewEngine(app.EngineParts{
		Dependencies:    deps,
		ContentConfig:   &content.Config{},
		StoryRepository: testutil.NewStoryRepository(),
		Scraper:         testutil.NewScraper(),
		Pipeline: &app.Pipeline{
			SchedulerConfig: &scheduler.Config{Interval: 0},
			GeneratorConfig: &generator.Config{},
			PosterConfig:    &poster.Config{},
			PostRepository:  testutil.NewPostRepository(),
			Narrator:        &testutil.Narrator{},
			Renderer:        &testutil.Renderer{},
			Platform:        &testutil.Platform{},
		},
	})

	require.Error(t, err)
}

func TestHealthEndpointReportsEachService(t *testing.T) {
	w := newWorld(t, time.Hour, true)
	server := httptest.NewServer(app.NewHealthHandler(w.engine))
	defer server.Close()

	resp, err := http.Get(server.URL + "/healthz")
	require.NoError(t, err)
	defer resp.Body.Close()

	var body struct {
		Healthy  bool            `json:"healthy"`
		Services map[string]bool `json:"services"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.True(t, body.Healthy)
	assert.Equal(t, map[string]bool{"Content": true, "Scheduler": true, "Generator": true, "Poster": true}, body.Services)
}

func TestHealthEndpointFailsWhenAServiceIsUnhealthy(t *testing.T) {
	w := newWorld(t, time.Hour, true)
	w.scraper.Unhealthy = true // e.g. Reddit is rejecting the login
	server := httptest.NewServer(app.NewHealthHandler(w.engine))
	defer server.Close()

	resp, err := http.Get(server.URL + "/healthz")
	require.NoError(t, err)
	defer resp.Body.Close()

	var body struct {
		Healthy  bool            `json:"healthy"`
		Services map[string]bool `json:"services"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	assert.False(t, body.Healthy)
	assert.False(t, body.Services["Content"])
	assert.True(t, body.Services["Poster"])
}

func TestLoadApplicationConfig(t *testing.T) {
	defaults, err := app.LoadConfig()
	require.NoError(t, err)
	assert.False(t, defaults.PipelineEnabled, "an unconfigured deployment only scrapes")
	assert.Equal(t, 8000, defaults.HTTPPort)

	t.Setenv("ENV", "dev")
	t.Setenv("PIPELINE_ENABLED", "true")
	t.Setenv("HTTP_PORT", "9000")
	cfg, err := app.LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, "dev", cfg.Env)
	assert.True(t, cfg.PipelineEnabled)
	assert.Equal(t, 9000, cfg.HTTPPort)
}

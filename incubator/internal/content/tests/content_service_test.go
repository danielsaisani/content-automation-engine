package tests

import (
	"content-automation-engine/internal/content"
	contentapi "content-automation-engine/internal/content/api"
	"content-automation-engine/internal/events"
	"content-automation-engine/internal/testutil"
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testConfig = &content.Config{ClaimTTL: 30 * time.Minute, MaxAttempts: 3, MinBodyChars: 10, MaxBodyChars: 1000}

const longBody = "This is a story body that is comfortably long enough to be selected."

type harness struct {
	service    *content.ContentService
	repository *testutil.StoryRepository
	scraper    *testutil.Scraper
	posted     *testutil.PostedStories
	topics     chan events.TopicTriggered
	selected   chan events.StorySelected
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	deps, _ := testutil.NewDependencies()
	h := &harness{
		repository: testutil.NewStoryRepository(),
		scraper:    testutil.NewScraper(),
		posted:     &testutil.PostedStories{},
		topics:     make(chan events.TopicTriggered, 10),
		selected:   make(chan events.StorySelected, 10),
	}
	h.service = content.NewContentService(deps, testConfig, h.topics, h.selected, h.repository, h.scraper, h.posted)
	return h
}

// run starts the service and stops it when the test ends.
func (h *harness) run(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- h.service.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		require.NoError(t, <-done)
	})
}

func (h *harness) storyCount() int { return len(h.repository.Stories()) }

func TestIngestStoresScrapedStories(t *testing.T) {
	h := newHarness(t)
	h.run(t)

	h.scraper.Emit(&contentapi.ScrapedContent{Source: "reddit", SourceID: "t3_1", Subreddit: "tifu", Title: "A title", Body: longBody, URL: "https://example.com/1"})

	require.True(t, testutil.Eventually(time.Second, func() bool { return h.storyCount() == 1 }))
	story := h.repository.Stories()[0]
	assert.Equal(t, "reddit", story.Source)
	assert.Equal(t, "t3_1", story.SourceID)
	assert.Equal(t, "tifu", story.Subreddit)
	assert.Equal(t, "A title", story.Title)
	assert.Equal(t, longBody, story.Body)
	assert.False(t, story.CreatedAt.IsZero(), "stories are stamped with the clock's time")
}

func TestIngestSkipsUnusablePosts(t *testing.T) {
	h := newHarness(t)
	h.run(t)

	h.scraper.Emit(nil)
	h.scraper.Emit(&contentapi.ScrapedContent{Source: "reddit", SourceID: "t3_nsfw", Title: "NSFW", Body: longBody, NSFW: true})
	h.scraper.Emit(&contentapi.ScrapedContent{Source: "reddit", SourceID: "t3_link", Title: "Link post", Body: "   "})
	h.scraper.Emit(&contentapi.ScrapedContent{Source: "reddit", SourceID: "t3_ok", Title: "Good", Body: longBody})

	require.True(t, testutil.Eventually(time.Second, func() bool { return h.storyCount() == 1 }))
	assert.Equal(t, "t3_ok", h.repository.Stories()[0].SourceID)
}

func TestIngestStoresEachPostOnce(t *testing.T) {
	h := newHarness(t)
	h.run(t)

	for range 3 {
		h.scraper.Emit(&contentapi.ScrapedContent{Source: "reddit", SourceID: "t3_same", Title: "Same", Body: longBody})
	}
	h.scraper.Emit(&contentapi.ScrapedContent{Source: "reddit", SourceID: "t3_other", Title: "Other", Body: longBody})

	require.True(t, testutil.Eventually(time.Second, func() bool { return h.storyCount() == 2 }))
	time.Sleep(20 * time.Millisecond)
	assert.Equal(t, 2, h.storyCount())
}

func TestIngestSurvivesRepositoryErrors(t *testing.T) {
	h := newHarness(t)
	h.repository.SetSaveErr(errors.New("database is down"))
	h.run(t)

	h.scraper.Emit(&contentapi.ScrapedContent{Source: "reddit", SourceID: "t3_1", Title: "Lost", Body: longBody})
	time.Sleep(20 * time.Millisecond)

	// The service is still running: once the database is back, stories are stored again.
	h.repository.SetSaveErr(nil)
	h.scraper.Emit(&contentapi.ScrapedContent{Source: "reddit", SourceID: "t3_2", Title: "Kept", Body: longBody})
	require.True(t, testutil.Eventually(time.Second, func() bool { return h.storyCount() == 1 }))
}

func TestSchedulerTriggerSelectsAStory(t *testing.T) {
	h := newHarness(t)
	id, _, err := h.repository.Save(context.Background(), contentapi.Story{Title: "Pick me", Body: longBody})
	require.NoError(t, err)
	h.run(t)

	h.topics <- events.TopicTriggered{Topic: "misc"}

	select {
	case selected := <-h.selected:
		assert.Equal(t, id, selected.StoryID)
		assert.NotEmpty(t, selected.ID, "events carry their own ID")
	case <-time.After(time.Second):
		t.Fatal("no StorySelected event was emitted")
	}
}

func TestSchedulerTriggerNeverSelectsTheSameStoryTwice(t *testing.T) {
	h := newHarness(t)
	_, _, err := h.repository.Save(context.Background(), contentapi.Story{Title: "Only one", Body: longBody})
	require.NoError(t, err)
	h.run(t)

	h.topics <- events.TopicTriggered{Topic: "misc"}
	h.topics <- events.TopicTriggered{Topic: "misc"}

	select {
	case <-h.selected:
	case <-time.After(time.Second):
		t.Fatal("no StorySelected event was emitted")
	}
	select {
	case second := <-h.selected:
		t.Fatalf("the claimed story was selected again: %+v", second)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestClaimStorySkipsPostedStories(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	older, _, _ := h.repository.Save(ctx, contentapi.Story{Title: "Older", Body: longBody})
	newer, _, _ := h.repository.Save(ctx, contentapi.Story{Title: "Newer", Body: longBody})
	h.posted.IDs = []string{newer}

	story, err := h.service.ClaimStory(ctx, "test")

	require.NoError(t, err)
	assert.Equal(t, older, story.ID, "the newest story is already posted, so the older one is claimed")
}

func TestClaimStoryRespectsBodyLengthLimits(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	_, _, _ = h.repository.Save(ctx, contentapi.Story{Title: "Too short", Body: "tiny"})
	_, _, _ = h.repository.Save(ctx, contentapi.Story{Title: "Too long", Body: strings.Repeat("x", 1001)})

	_, err := h.service.ClaimStory(ctx, "test")
	require.ErrorIs(t, err, contentapi.ErrNoStoryAvailable)

	fits, _, _ := h.repository.Save(ctx, contentapi.Story{Title: "Just right", Body: longBody})
	story, err := h.service.ClaimStory(ctx, "test")
	require.NoError(t, err)
	assert.Equal(t, fits, story.ID)
}

func TestClaimedStoryBecomesAvailableAgainAfterTheClaimExpires(t *testing.T) {
	deps, mockClock := testutil.NewDependencies()
	repository := testutil.NewStoryRepository()
	service := content.NewContentService(deps, testConfig, nil, nil, repository, testutil.NewScraper(), nil)
	ctx := context.Background()
	id, _, _ := repository.Save(ctx, contentapi.Story{Title: "Retry me", Body: longBody})

	first, err := service.ClaimStory(ctx, "test")
	require.NoError(t, err)
	assert.Equal(t, id, first.ID)

	_, err = service.ClaimStory(ctx, "test")
	require.ErrorIs(t, err, contentapi.ErrNoStoryAvailable, "the story is held until its claim expires")

	mockClock.Advance(testConfig.ClaimTTL + time.Second)
	again, err := service.ClaimStory(ctx, "test")
	require.NoError(t, err)
	assert.Equal(t, id, again.ID)
}

func TestStoryIsGivenUpOnAfterMaxAttempts(t *testing.T) {
	deps, mockClock := testutil.NewDependencies()
	repository := testutil.NewStoryRepository()
	service := content.NewContentService(deps, testConfig, nil, nil, repository, testutil.NewScraper(), nil)
	ctx := context.Background()
	_, _, _ = repository.Save(ctx, contentapi.Story{Title: "Always fails", Body: longBody})

	for attempt := range testConfig.MaxAttempts {
		_, err := service.ClaimStory(ctx, "test")
		require.NoError(t, err, "attempt %d", attempt+1)
		mockClock.Advance(testConfig.ClaimTTL + time.Second)
	}

	_, err := service.ClaimStory(ctx, "test")
	require.ErrorIs(t, err, contentapi.ErrNoStoryAvailable)
}

func TestConcurrentClaimsNeverReturnTheSameStory(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	const stories, callers = 5, 20
	for range stories {
		_, _, _ = h.repository.Save(ctx, contentapi.Story{Title: "Story", Body: longBody})
	}

	var mu sync.Mutex
	claimed := make(map[string]int)
	var wg sync.WaitGroup
	for range callers {
		wg.Go(func() {
			story, err := h.service.ClaimStory(ctx, "test")
			if err != nil {
				return
			}
			mu.Lock()
			claimed[story.ID]++
			mu.Unlock()
		})
	}
	wg.Wait()

	assert.Len(t, claimed, stories, "every story is claimed by someone")
	for id, count := range claimed {
		assert.Equal(t, 1, count, "story %s was handed out more than once", id)
	}
}

func TestClaimStoryFailsWhenPostedStoriesCannotBeListed(t *testing.T) {
	h := newHarness(t)
	_, _, _ = h.repository.Save(context.Background(), contentapi.Story{Title: "Story", Body: longBody})
	h.posted.Err = errors.New("poster database is down")

	_, err := h.service.ClaimStory(context.Background(), "test")

	// Without knowing what has been posted, claiming could cause a repost, so it must fail.
	require.Error(t, err)
	assert.NotErrorIs(t, err, contentapi.ErrNoStoryAvailable)
}

func TestGetStoryByID(t *testing.T) {
	h := newHarness(t)
	id, _, _ := h.repository.Save(context.Background(), contentapi.Story{Title: "Find me", Body: longBody})

	story, err := h.service.GetStoryByID(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, "Find me", story.Title)

	_, err = h.service.GetStoryByID(context.Background(), "missing")
	require.ErrorIs(t, err, contentapi.ErrStoryNotFound)
}

func TestHealthFollowsRepositoryAndScraper(t *testing.T) {
	h := newHarness(t)
	assert.True(t, h.service.Healthy(context.Background()))

	h.scraper.Unhealthy = true
	assert.False(t, h.service.Healthy(context.Background()))

	h.scraper.Unhealthy = false
	h.repository.Unhealthy = true
	assert.False(t, h.service.Healthy(context.Background()))
}

func TestLoadConfigDefaults(t *testing.T) {
	cfg, err := content.LoadConfig()

	require.NoError(t, err)
	assert.Equal(t, 30*time.Minute, cfg.ClaimTTL)
	assert.Equal(t, 3, cfg.MaxAttempts)
}

func TestLoadConfigFromEnvironment(t *testing.T) {
	t.Setenv("CONTENT_CLAIM_TTL", "45m")
	t.Setenv("CONTENT_MIN_BODY_CHARS", "50")

	cfg, err := content.LoadConfig()

	require.NoError(t, err)
	assert.Equal(t, 45*time.Minute, cfg.ClaimTTL)
	assert.Equal(t, 50, cfg.MinBodyChars)
}

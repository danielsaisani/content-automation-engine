package tests

import (
	"content-automation-engine/internal/content/scraper"
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/turnage/graw/reddit"
)

var silentLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

// fakeSessions hands out scraping sessions that tests can fail on demand.
type fakeSessions struct {
	mu       sync.Mutex
	started  int
	startErr error
	failures chan error
	stopped  chan struct{}
}

func newFakeSessions() *fakeSessions {
	return &fakeSessions{failures: make(chan error, 10), stopped: make(chan struct{}, 10)}
}

func (f *fakeSessions) start(handler *scraper.Handler) (*scraper.Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.started++
	if f.startErr != nil {
		return nil, f.startErr
	}

	done := make(chan struct{})
	return &scraper.Session{
		Stop: func() { close(done); f.stopped <- struct{}{} },
		Wait: func() error {
			select {
			case err := <-f.failures:
				return err
			case <-done:
				return nil
			}
		},
	}, nil
}

func (f *fakeSessions) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.started
}

func TestNormalizeSubreddits(t *testing.T) {
	got := scraper.NormalizeSubreddits([]string{"AskReddit", "/r/tifu", "r/AITAH", " r/self ", "", "  "})

	assert.Equal(t, []string{"AskReddit", "tifu", "AITAH", "self"}, got)
}

func TestHandlerForwardsPostsWithTheirRedditIdentity(t *testing.T) {
	s := scraper.NewRedditScraperWithStarter(silentLogger, []string{"tifu"}, newFakeSessions().start)

	err := s.Handler().Post(&reddit.Post{Name: "t3_abc", Subreddit: "tifu", Title: "TIFU", SelfText: "The story", URL: "https://reddit.com/abc", NSFW: true})
	require.NoError(t, err)

	post := <-s.Posts()
	assert.Equal(t, "reddit", post.Source)
	assert.Equal(t, "t3_abc", post.SourceID)
	assert.Equal(t, "tifu", post.Subreddit)
	assert.Equal(t, "TIFU", post.Title)
	assert.Equal(t, "The story", post.Body)
	assert.True(t, post.NSFW)
}

func TestScraperRestartsAfterASessionFails(t *testing.T) {
	sessions := newFakeSessions()
	s := scraper.NewRedditScraperWithStarter(silentLogger, []string{"tifu"}, sessions.start)
	var waits []time.Duration
	var mu sync.Mutex
	s.SetSleep(func(ctx context.Context, d time.Duration) {
		mu.Lock()
		waits = append(waits, d)
		mu.Unlock()
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()

	require.True(t, eventually(func() bool { return sessions.count() == 1 && s.Healthy(ctx) }))

	// Reddit rejects us: the session dies, and a new one must be started.
	sessions.failures <- errors.New("unauthorized access to endpoint")
	require.True(t, eventually(func() bool { return sessions.count() == 2 && s.Healthy(ctx) }))

	sessions.failures <- errors.New("unauthorized access to endpoint")
	require.True(t, eventually(func() bool { return sessions.count() == 3 }))

	cancel()
	require.NoError(t, <-done, "a cancelled scraper stops cleanly")
	assert.False(t, s.Healthy(context.Background()))

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, waits, 2)
	assert.Equal(t, 2*waits[0], waits[1], "the wait doubles after each consecutive failure")
}

func TestScraperKeepsRetryingWhenLoginFails(t *testing.T) {
	sessions := newFakeSessions()
	sessions.startErr = errors.New("logging in to reddit: 401")
	s := scraper.NewRedditScraperWithStarter(silentLogger, []string{"tifu"}, sessions.start)
	s.SetSleep(func(ctx context.Context, d time.Duration) { time.Sleep(time.Millisecond) })

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()

	require.True(t, eventually(func() bool { return sessions.count() >= 3 }))
	assert.False(t, s.Healthy(ctx), "a scraper that cannot log in is unhealthy")

	cancel()
	require.NoError(t, <-done)
}

func TestScraperStopsItsSessionWhenCancelled(t *testing.T) {
	sessions := newFakeSessions()
	s := scraper.NewRedditScraperWithStarter(silentLogger, []string{"tifu"}, sessions.start)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()
	require.True(t, eventually(func() bool { return s.Healthy(ctx) }))

	cancel()

	require.NoError(t, <-done)
	select {
	case <-sessions.stopped:
	case <-time.After(time.Second):
		t.Fatal("the session was not stopped")
	}
	assert.Equal(t, 1, sessions.count(), "no new session is started after cancellation")
}

func TestLoadRedditScraperConfig(t *testing.T) {
	t.Setenv("SCRAPER_APP_ID", "client-id")
	t.Setenv("SCRAPER_APP_SECRET", "client-secret")
	t.Setenv("SCRAPER_USERNAME", "someone")
	t.Setenv("SCRAPER_PASSWORD", "hunter2")
	t.Setenv("SCRAPER_SUBREDDITS", "tifu,AITAH,r/self")

	cfg, err := scraper.LoadRedditScraperConfig()

	require.NoError(t, err)
	assert.Equal(t, "client-id", cfg.AppID)
	assert.Equal(t, "client-secret", cfg.AppSecret)
	assert.Equal(t, []string{"tifu", "AITAH", "r/self"}, cfg.Subreddits)
	assert.Equal(t, 5*time.Second, cfg.PollInterval)
}

func TestLoadRedditScraperConfigRequiresCredentials(t *testing.T) {
	t.Setenv("SCRAPER_APP_ID", "client-id")

	_, err := scraper.LoadRedditScraperConfig()

	require.Error(t, err)
}

func eventually(check func() bool) bool {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if check() {
			return true
		}
		time.Sleep(2 * time.Millisecond)
	}
	return check()
}

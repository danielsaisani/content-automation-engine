package scraper

import (
	contentapi "content-automation-engine/internal/content/api"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"github.com/turnage/graw"
	"github.com/turnage/graw/reddit"
	"github.com/vrischmann/envconfig"
)

// DefaultSubreddits is scraped when SCRAPER_SUBREDDITS is not set.
var DefaultSubreddits = []string{
	"worldnews", "news", "politics", "europe", "ukpolitics",
	"conservative", "liberal", "geopolitics", "upliftingnews", "nottheonion",
	"nfl", "nba", "soccer", "baseball", "hockey",
	"formula1", "tennis", "MMA", "cricket", "fantasyfootball",
	"gaming", "pcgaming", "PS5", "XboxSeriesX", "leagueoflegends",
	"Minecraft", "Fortnite", "GlobalOffensive", "Competitiveoverwatch", "DotA2",
	"technology", "programming", "Python", "javascript", "webdev",
	"artificial", "MachineLearning", "ChatGPT", "cybersecurity", "linux",
	"movies", "television", "anime", "marvelstudios", "StarWars",
	"NetflixBestOf", "popheads", "hiphopheads", "music", "books",
	"AskReddit", "AmItheAsshole", "relationship_advice", "dating_advice", "tifu",
	"confession", "unpopularopinion", "changemyview", "LifeProTips", "self",
	"wallstreetbets", "investing", "personalfinance", "CryptoCurrency", "Bitcoin",
	"ethereum", "stocks", "financialindependence", "frugal", "eupersonalfinance",
	"memes", "dankmemes", "funny", "me_irl", "facepalm",
	"therewasanattempt", "Unexpected", "instant_regret", "nextfuckinglevel", "oddlysatisfying",
	"science", "space", "history", "todayilearned", "explainlikeimfive",
	"askscience", "medicine", "psychology", "philosophy", "economics",
	"pics", "gifs", "videos", "WTF", "interestingasfuck",
	"mildlyinteresting", "photoshopbattles", "DIY", "food", "aww", "AITAH",
}

// RedditScraperConfig is read from environment variables prefixed with SCRAPER_.
type RedditScraperConfig struct {
	// AppID and AppSecret identify the registered Reddit app (its client ID and secret).
	AppID     string
	AppSecret string
	// Username and Password are the Reddit account the app logs in as. For a "script" app
	// the account must be listed as a developer of the app.
	Username string
	Password string
	// Subreddits is a comma-separated list. When empty, DefaultSubreddits is used.
	Subreddits []string `envconfig:"optional"`
	UserAgent  string   `envconfig:"default=content-automation-engine/0.1"`
	// PollInterval is the minimum time between requests to Reddit.
	PollInterval time.Duration `envconfig:"default=5s"`
}

// LoadRedditScraperConfig reads the scraper configuration from the environment.
func LoadRedditScraperConfig() (*RedditScraperConfig, error) {
	cfg := &RedditScraperConfig{}
	if err := envconfig.InitWithPrefix(cfg, "SCRAPER"); err != nil {
		return nil, fmt.Errorf("loading scraper config: %w", err)
	}
	return cfg, nil
}

// normalizeSubredditName returns the name graw expects: no leading slashes and no "r/" prefix.
// graw builds paths as "/r/" + join(names, "+") + "/new", so "/r/foo" would become "/r//r/foo/new".
func normalizeSubredditName(name string) string {
	s := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(name), "/"))
	if strings.HasPrefix(strings.ToLower(s), "r/") {
		s = s[2:]
	}
	return strings.TrimPrefix(strings.TrimSpace(s), "/")
}

// NormalizeSubreddits cleans a list of subreddit names, dropping empty entries.
func NormalizeSubreddits(subreddits []string) []string {
	out := make([]string, 0, len(subreddits))
	for _, s := range subreddits {
		if n := normalizeSubredditName(s); n != "" {
			out = append(out, n)
		}
	}
	return out
}

// Handler receives posts from graw and forwards them to the scraper's channel.
type Handler struct {
	logger *slog.Logger
	posts  chan *contentapi.ScrapedContent
}

func (h *Handler) Post(post *reddit.Post) error {
	h.logger.Debug("Received new post", "subreddit", post.Subreddit, "title", post.Title)
	h.posts <- &contentapi.ScrapedContent{
		Source:    "reddit",
		SourceID:  post.Name,
		Subreddit: post.Subreddit,
		Title:     post.Title,
		Body:      post.SelfText,
		URL:       post.URL,
		NSFW:      post.NSFW,
	}
	return nil
}

// Session is one running connection to the source. Stop ends it; Wait blocks until it fails.
type Session struct {
	Stop func()
	Wait func() error
}

// SessionStarter opens a scraping session that feeds posts to the handler.
// It exists so tests can replace the real Reddit connection.
type SessionStarter func(handler *Handler) (*Session, error)

const (
	initialBackoff = 5 * time.Second
	maxBackoff     = 5 * time.Minute
	// A session that survived this long is considered to have been working, so the next
	// failure starts again from the initial backoff.
	stableAfter = 2 * time.Minute
)

var _ contentapi.Scraper = (*RedditScraper)(nil)

type RedditScraper struct {
	logger     *slog.Logger
	subreddits []string
	handler    *Handler
	start      SessionStarter
	connected  atomic.Bool
	// sleep waits for d or until ctx is cancelled; replaced in tests.
	sleep func(ctx context.Context, d time.Duration)
}

// NewRedditScraper builds a scraper that logs in to Reddit with the configured app.
func NewRedditScraper(logger *slog.Logger, cfg *RedditScraperConfig) *RedditScraper {
	subreddits := NormalizeSubreddits(cfg.Subreddits)
	if len(subreddits) == 0 {
		subreddits = NormalizeSubreddits(DefaultSubreddits)
	}

	starter := func(handler *Handler) (*Session, error) {
		bot, err := reddit.NewBot(
			reddit.BotConfig{
				Agent: cfg.UserAgent,
				App: reddit.App{
					ID:       cfg.AppID,
					Secret:   cfg.AppSecret,
					Username: cfg.Username,
					Password: cfg.Password,
				},
				Rate: cfg.PollInterval,
			},
		)
		if err != nil {
			return nil, fmt.Errorf("logging in to reddit: %w", err)
		}

		stop, wait, err := graw.Run(handler, bot, graw.Config{Subreddits: subreddits})
		if err != nil {
			return nil, fmt.Errorf("starting reddit stream: %w", err)
		}
		return &Session{Stop: stop, Wait: wait}, nil
	}

	return NewRedditScraperWithStarter(logger, subreddits, starter)
}

// NewRedditScraperWithStarter builds a scraper around a custom session starter.
func NewRedditScraperWithStarter(logger *slog.Logger, subreddits []string, starter SessionStarter) *RedditScraper {
	logger = logger.With("service", "scraper")
	return &RedditScraper{
		logger:     logger,
		subreddits: subreddits,
		handler:    &Handler{logger: logger, posts: make(chan *contentapi.ScrapedContent, 10)},
		start:      starter,
		sleep:      sleepContext,
	}
}

// SetSleep replaces how the scraper waits between retries. Intended for tests.
func (rc *RedditScraper) SetSleep(sleep func(ctx context.Context, d time.Duration)) {
	rc.sleep = sleep
}

func (rc *RedditScraper) Posts() <-chan *contentapi.ScrapedContent {
	return rc.handler.posts
}

// Handler returns the handler sessions deliver posts to.
func (rc *RedditScraper) Handler() *Handler {
	return rc.handler
}

// Healthy reports whether a scraping session is currently running.
func (rc *RedditScraper) Healthy(ctx context.Context) bool {
	return rc.connected.Load()
}

// Run keeps a scraping session alive until ctx is cancelled. When a session fails (Reddit
// rejects the login, rate limits us, the network drops...) the failure is logged and a new
// session is started after a backoff, so one bad response never silently stops scraping.
func (rc *RedditScraper) Run(ctx context.Context) error {
	rc.logger.Info("Starting reddit scraper", "subreddits", len(rc.subreddits))
	backoff := initialBackoff

	for {
		startedAt := time.Now()
		err := rc.runSession(ctx)
		if ctx.Err() != nil {
			rc.logger.Info("Reddit scraper stopped")
			return nil
		}

		if time.Since(startedAt) >= stableAfter {
			backoff = initialBackoff
		}
		rc.logger.Error("Reddit scraping session failed, will retry", "err", err, "retryIn", backoff.String())
		rc.sleep(ctx, backoff)
		backoff = min(backoff*2, maxBackoff)
	}
}

// runSession runs one session until it fails or ctx is cancelled.
func (rc *RedditScraper) runSession(ctx context.Context) error {
	session, err := rc.start(rc.handler)
	if err != nil {
		return err
	}

	rc.connected.Store(true)
	defer rc.connected.Store(false)
	rc.logger.Info("Reddit scraping session started")

	failed := make(chan error, 1)
	go func() { failed <- session.Wait() }()

	select {
	case <-ctx.Done():
		session.Stop()
		return nil
	case err := <-failed:
		if err == nil {
			return errors.New("session ended unexpectedly")
		}
		return err
	}
}

func sleepContext(ctx context.Context, d time.Duration) {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
	}
}

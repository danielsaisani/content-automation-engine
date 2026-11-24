package scraper

import (
	contentapi "content-automation-engine/internal/content/api"
	"context"
	"fmt"
	"log"
	"log/slog"
	"strings"
	"time"

	"github.com/turnage/graw"
	"github.com/turnage/graw/reddit"
	"github.com/vrischmann/envconfig"
)

type RedditConfig struct {
	AppID     string
	AppSecret string
	Username  string
	Password  string
}

type Config struct {
	RC *RedditConfig
	SC *graw.Config
}

// normalizeSubredditName returns the name graw expects: no leading slashes and no "r/" prefix.
// graw builds paths as "/r/" + join(names, "+") + "/new", so "/r/foo" would become "/r//r/foo/new".
func normalizeSubredditName(name string) string {
	s := strings.TrimSpace(strings.TrimPrefix(name, "/"))
	if strings.HasPrefix(strings.ToLower(s), "r/") {
		s = s[2:]
	}
	return strings.TrimPrefix(strings.TrimSpace(s), "/")
}

// NewRedditScraperConfig creates a new RedditScraperConfig with the given subreddits
func NewRedditScraperConfig(subreddits []string) *Config {
	grawCfg := &graw.Config{}
	redditConfig := &RedditConfig{}
	cfg := &Config{
		SC: grawCfg,
		RC: redditConfig,
	}

	out := make([]string, 0, len(subreddits))
	for _, s := range subreddits {
		if n := normalizeSubredditName(s); n != "" {
			out = append(out, n)
		}
	}
	grawCfg.Subreddits = out

	if err := envconfig.InitWithPrefix(redditConfig, "SCRAPER"); err != nil {
		log.Fatal("Error loading config:", err)
	}

	return cfg
}

type Handler struct {
	posts chan *contentapi.ScrapedContent
}

func (h *Handler) Post(post *reddit.Post) error {
	slog.Info(fmt.Sprintf("Received new post from subreddit. Title: %s", post.Title))
	h.posts <- &contentapi.ScrapedContent{
		Title: post.Title,
		Body:  post.SelfText,
		NSFW:  post.NSFW,
	}
	return nil
}

type RedditScraper struct {
	config  *Config
	handler *Handler
}

func NewRedditScraper(subreddits []string) *RedditScraper {
	return &RedditScraper{
		config:  NewRedditScraperConfig(subreddits),
		handler: &Handler{posts: make(chan *contentapi.ScrapedContent, 10)},
	}
}

func (rc *RedditScraper) Posts() <-chan *contentapi.ScrapedContent {
	return rc.handler.posts
}

func (rc *RedditScraper) Run(ctx context.Context) error {
	// The unathenticated Reddit API has a rate limit of 30 requests per minute, so we're being cautious and doing 12 requests per minute.
	bot, err := reddit.NewBot(
		reddit.BotConfig{
			Agent: "content-automation-engine/0.1",
			App: reddit.App{
				ID:       rc.config.RC.AppID,
				Secret:   rc.config.RC.AppSecret,
				Username: rc.config.RC.Username,
				Password: rc.config.RC.Password,
			},
			Rate: 5 * time.Second,
		},
	)

	// TODO: use the returned wait function to handle errors on the graw poller side
	stop, _, err := graw.Run(rc.handler, bot, *rc.config.SC)
	if err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		stop()
	}()

	return nil
}

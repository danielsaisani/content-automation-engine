package scraper

import (
	creatorapi "content-automation-engine/internal/creator/api"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/turnage/graw"
	"github.com/turnage/graw/reddit"
)

type RedditScraperConfig struct {
	Engine *graw.Config
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
func NewRedditScraperConfig(subreddits []string) *RedditScraperConfig {
	out := make([]string, 0, len(subreddits))
	for _, s := range subreddits {
		if n := normalizeSubredditName(s); n != "" {
			out = append(out, n)
		}
	}
	return &RedditScraperConfig{&graw.Config{Subreddits: out}}
}

type Handler struct {
	posts chan *creatorapi.ScrapedContent
}

func (h *Handler) Post(post *reddit.Post) error {
	slog.Info(fmt.Sprintf("Received new post from subreddit. Title: %s", post.Title))
	h.posts <- &creatorapi.ScrapedContent{
		Title: post.Title,
		Body:  post.SelfText,
		NSFW:  post.NSFW,
	}
	return nil
}

type RedditScraper struct {
	config  *RedditScraperConfig
	handler *Handler
}

func NewRedditScraper(subreddits []string) *RedditScraper {
	return &RedditScraper{
		config:  NewRedditScraperConfig(subreddits),
		handler: &Handler{posts: make(chan *creatorapi.ScrapedContent, 10)},
	}
}

func (rc *RedditScraper) Posts() <-chan *creatorapi.ScrapedContent {
	return rc.handler.posts
}

func (rc *RedditScraper) Run(ctx context.Context) error {
	script, err := reddit.NewScript("content-automation-engine/0.1", 5*time.Second)
	if err != nil {
		return err
	}

	stop, _, err := graw.Scan(rc.handler, script, *rc.config.Engine)
	if err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		stop()
	}()

	return nil
}

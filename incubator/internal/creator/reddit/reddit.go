package reddit

import (
	"content-automation-engine/internal/api"
	"context"
	"time"

	"github.com/turnage/graw"
	"github.com/turnage/graw/reddit"
)

type RedditScraperConfig struct {
	Engine *graw.Config
}

// NewRedditScraperConfig creates a new RedditScraperConfig with the given subreddits
func NewRedditScraperConfig(subreddits []string) *RedditScraperConfig {
	return &RedditScraperConfig{&graw.Config{Subreddits: subreddits}}
}

type Handler struct {
	posts chan *api.Story
}

func (h *Handler) Post(post *reddit.Post) error {
	h.posts <- &api.Story{Title: post.Title, Body: post.SelfText}
	return nil
}

type RedditScraper struct {
	config  *RedditScraperConfig
	handler *Handler
}

func NewRedditScraper(subreddits []string) *RedditScraper {
	return &RedditScraper{
		config:  NewRedditScraperConfig(subreddits),
		handler: &Handler{posts: make(chan *api.Story, 10)},
	}
}

func (rc *RedditScraper) Posts() <-chan *api.Story {
	return rc.handler.posts
}

func (rc *RedditScraper) Run(ctx context.Context) error {
	script, err := reddit.NewScript("content-automation-engine/0.1", 5*time.Second)
	if err != nil {
		return err
	}

	stop, wait, err := graw.Scan(rc.handler, script, *rc.config.Engine)
	if err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		stop()
	}()

	return wait()
}

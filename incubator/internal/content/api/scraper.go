package api

import (
	"context"
	"time"

	"github.com/turnage/graw/reddit"
)

type Scraper interface {
	Posts() <-chan *ScrapedContent // CreatorService reads from this and pushes it to the StoryRepository
	Run(ctx context.Context) error
}

// ScrapedContent is the internal model of the stories that we scrape from it's source
type ScrapedContent struct {
	ID           string
	Name         string
	Title        string
	Body         string
	NSFW         bool
	Likes        uint
	Dislikes     uint
	CommentCount uint32
	Comments     []*reddit.Comment
	Media        reddit.Media
	Subreddit    string
	SubredditID  string
	// I wonder what this is...?
	Score      int32
	Author     string
	CreatedUTC time.Time
}

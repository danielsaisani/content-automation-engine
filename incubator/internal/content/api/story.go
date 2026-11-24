package api

import (
	"errors"
	"time"
)

var (
	// ErrNoStoryAvailable means every stored story is claimed, posted, unsuitable or out of attempts.
	ErrNoStoryAvailable = errors.New("no story available")
	// ErrStoryNotFound means no story has the requested ID.
	ErrStoryNotFound = errors.New("story not found")
)

// Story is a piece of scraped content that can be turned into a post.
type Story struct {
	// ID is the repository's identifier for the story.
	ID string
	// Source names where the story came from, e.g. "reddit".
	Source string
	// SourceID is the story's identifier at its source (for Reddit, the post's fullname such
	// as "t3_abc123"). It stops the same post being stored twice.
	SourceID  string
	Subreddit string
	Title     string
	Body      string
	URL       string
	CreatedAt time.Time
}

// ScrapedContent is what a Scraper hands to the content service.
type ScrapedContent struct {
	Source    string
	SourceID  string
	Subreddit string
	Title     string
	Body      string
	URL       string
	NSFW      bool
}

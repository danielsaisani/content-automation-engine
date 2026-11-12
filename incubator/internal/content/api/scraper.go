package api

import "context"

type Scraper interface {
	Posts() <-chan *ScrapedContent // CreatorService reads from this and pushes it to the StoryRepository
	Run(ctx context.Context) error
}

type ScrapedContent struct {
	Title string
	Body  string
	NSFW  bool
}

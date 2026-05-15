package api

import "context"

type Scraper interface {
	Posts() <-chan *ScrapedContent // CreatorService reads from this and pushes it to the StoryRepository
	Run(ctx context.Context) error
}

type ScrapedContent struct {
	Title  string
	Body   ScrapedContentBody
	NSFW   bool
	Posted bool
}

type ScrapedContentBody struct {
	Body string
}

// Populated returns whether the body actually exists to distinguish between posts that don't have any content and those that do.
func (sb ScrapedContentBody) Populated() bool {
	return len(sb.Body) > 0
}

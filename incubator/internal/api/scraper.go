package api

import "context"

type Scraper interface {
	Posts() <-chan *Story // CreatorService reads from this and pushes it to the StoryRepository
	Run(ctx context.Context) error
}

type Story struct {
	Title  string
	Body   StoryBody
	NSFW   bool
	Posted bool
}

type StoryBody struct {
	Body string
}

// Populated returns whether the body actually exists to distinguish between posts that don't have any content and those that do.
func (sb StoryBody) Populated() bool {
	return len(sb.Body) > 0
}

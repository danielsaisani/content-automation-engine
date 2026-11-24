package api

import "context"

type Scraper interface {
	// Posts is the stream of scraped content. The content service reads from this and
	// pushes it to the StoryRepository.
	Posts() <-chan *ScrapedContent
	// Run scrapes until ctx is cancelled. It blocks, and recovers from failures by itself.
	Run(ctx context.Context) error
	// Healthy reports whether the scraper is currently connected to its source.
	Healthy(ctx context.Context) bool
}

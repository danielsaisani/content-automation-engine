package api

import "context"

type Scraper interface {
	Posts() <-chan *Story // CreatorService reads from this
	Run(ctx context.Context) error
}

type Story struct {
	Title string
	Body  string
}

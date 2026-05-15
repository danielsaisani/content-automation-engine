package api

import (
	sharedapi "content-automation-engine/internal/api"
)

type StorySearchCriteria struct {
	Topic string
}

type StoryObjectRepository interface {
	sharedapi.ObjectRepository
	// Method to get best story matching
	GetBestStory(StorySearchCriteria) (string, error)
}

type Story struct {
	Title string
	Body  string
}

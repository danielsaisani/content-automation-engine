package api

import (
	sharedAPI "content-automation-engine/internal/api"
)

type CreatorService interface {
	sharedAPI.Service
	GetStoryByID(id string) (Story, error)
}

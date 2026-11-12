package api

import (
	sharedAPI "content-automation-engine/internal/api"
	"context"
)

type CreatorService interface {
	sharedAPI.Service
	GetStoryByID(ctx context.Context, id string) (Story, error)
}

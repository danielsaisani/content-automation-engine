package app

import (
	creator "content-automation-engine/internal/creator/repository"
	"context"
	"fmt"
	"os"
)

type ContentAutomationEngineApplication struct{}

func (a *ContentAutomationEngineApplication) Start(ctx context.Context) error {
	var mongoRepositoryConfig *creator.MongoStoryRepositoryConfig

	mongoRepositoryConfig = creator.NewMongoStoryRepositoryConfig(os.Getenv("MONGO_USERNAME"), os.Getenv("MONGO_PASSWORD"), os.Getenv("MONGO_HOST"), os.Getenv("MONGO_APP_NAME"), os.Getenv("ENV"))

	repo := creator.NewMongoStoryRepository(mongoRepositoryConfig)
	if err := repo.InitialiseClient(ctx); err != nil {
		return fmt.Errorf("story repository: %w", err)
	}

	return nil
}

func (a *ContentAutomationEngineApplication) Shutdown(ctx context.Context) error { return nil }

func (a *ContentAutomationEngineApplication) Healthy(ctx context.Context) bool { return true }

func (a *ContentAutomationEngineApplication) Name(ctx context.Context) string {
	return "ContentAutomationEngineApplication"
}
func (a *ContentAutomationEngineApplication) Port(ctx context.Context) int { return 8080 }

package config

import (
	creator "content-automation-engine/internal/creator/repository"
	"context"
	"fmt"
	"os"
)

type Config struct {
	StoryRepository *creator.MongoStoryRepository
}

func NewConfig() *Config {
	return &Config{}
}

// Load injects the configuration in the environment into the struct to be used in the application
func (c *Config) Load(ctx context.Context) error {
	var mongoRepositoryConfig *creator.MongoStoryRepositoryConfig

	mongoRepositoryConfig = creator.NewMongoStoryRepositoryConfig(os.Getenv("MONGO_USERNAME"), os.Getenv("MONGO_PASSWORD"), os.Getenv("MONGO_HOST"), os.Getenv("MONGO_APP_NAME"), os.Getenv("ENV"))

	repo := creator.NewMongoStoryRepository(mongoRepositoryConfig)
	if err := repo.InitialiseClient(ctx); err != nil {
		return fmt.Errorf("story repository: %w", err)
	}
	c.StoryRepository = repo
	return nil
}

package config

import (
	creator "content-automation-engine/internal/creator/repository"
	"context"
	"os"
)

type Config struct {
	StoryRepository *creator.MongoStoryRepository
}

func NewConfig() *Config {
	return &Config{}
}

// Load injects the configuration in the environment into the struct to be used in the application
func (c *Config) Load(ctx context.Context) {
	mongoRepositoryConfig := creator.NewMongoStoryRepositoryConfig(os.Getenv("MONGO_USERNAME"), os.Getenv("MONGO_PASSWORD"), os.Getenv("MONGO_APP_NAME"))
	c.StoryRepository = creator.NewMongoStoryRepository(mongoRepositoryConfig)
}

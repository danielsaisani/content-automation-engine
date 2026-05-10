package config

import (
	"content-automation-engine/internal/creator"
	"context"
	"os"
)

type Config struct {
	StoryRepository *creator.MongoStoryRepository
}

func NewConfig() *Config {
	return &Config{}
}

func (c *Config) Load(ctx context.Context) {
	mongoRepositoryConfig := creator.NewMongoStoryRepositoryConfig(os.Getenv("MONGO_USERNAME"), os.Getenv("MONGO_PASSWORD"), os.Getenv("MONGO_APP_NAME"))
	c.StoryRepository = creator.NewMongoStoryRepository(mongoRepositoryConfig)
}

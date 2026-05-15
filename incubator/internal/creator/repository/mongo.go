package creator

import (
	"content-automation-engine/internal/creator/api"
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

type MongoStoryRepositoryConfig struct {
	ConnectionURL string
}

func NewMongoStoryRepositoryConfig(mongoUsername string, mongoPassword string, mongoApp string) *MongoStoryRepositoryConfig {
	// mongodb+srv://<username>:<password>@prod.skbzy7n.mongodb.net/?appName=prod
	connectionURL := fmt.Sprintf("mongodb+srv://%s:%s@prod.skbzy7n.mongodb.net/?appName=%s", mongoUsername, mongoPassword, mongoApp)
	return &MongoStoryRepositoryConfig{ConnectionURL: connectionURL}
}

type MongoStoryRepository struct {
	Config *MongoStoryRepositoryConfig
	Client *mongo.Client
}

func NewMongoStoryRepository(repositoryConfig *MongoStoryRepositoryConfig) *MongoStoryRepository {
	return &MongoStoryRepository{
		Config: repositoryConfig,
	}
}

// Gets the story by the the ID
func (mr *MongoStoryRepository) Get(searchCritera interface{}) (interface{}, error) {
	var story api.Story

	searchResult := mr.Client.Database("prod").Collection("stories").FindOne(context.TODO(), searchCritera)
	if searchResultErr := searchResult.Err(); searchResultErr != nil {
		return nil, searchResultErr
	}
	if err := searchResult.Decode(&story); err != nil {
		return nil, err
	}

	return story, nil
}

func (mr *MongoStoryRepository) Put(story interface{}) (bool, error) {
	if mr.Client == nil {
		return false, errors.New("mongodb client not initialised")
	}
	_, err := mr.Client.Database("prod").Collection("stories").InsertOne(context.TODO(), story)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (mr *MongoStoryRepository) GetBestStory(criteria api.StorySearchCriteria) (string, error) {
	return "", nil
}

func (mr *MongoStoryRepository) InitialiseClient(ctx context.Context) error {
	if mr.Config == nil || mr.Config.ConnectionURL == "" {
		return errors.New("missing mongodb connection url")
	}

	serverAPI := options.ServerAPI(options.ServerAPIVersion1)
	opts := options.Client().ApplyURI(mr.Config.ConnectionURL).SetServerAPIOptions(serverAPI)

	client, err := mongo.Connect(opts)
	if err != nil {
		return err
	}

	if err = client.Ping(ctx, readpref.Primary()); err != nil {
		_ = client.Disconnect(ctx)
		return err
	}

	mr.Client = client
	return nil
}

// Performs a health check on the StoryRepository
func (mr *MongoStoryRepository) Healthy(ctx context.Context) bool {
	if mr.Client == nil {
		return false
	}

	if err := mr.Client.Ping(ctx, readpref.Primary()); err != nil {
		return false
	}
	return true
}

package creator

import (
	"content-automation-engine/internal/creator/api"
	"context"
	"errors"
	"strings"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

type ConnectionURLBuilder struct {
	lines []string
}

func (cb *ConnectionURLBuilder) Method(env string) *ConnectionURLBuilder {
	cb.lines = append(cb.lines, "mongodb")
	if env != "dev" {
		cb.lines = append(cb.lines, "+srv")
	}
	cb.lines = append(cb.lines, "://")
	return cb
}

func (cb *ConnectionURLBuilder) Credentials(username string, password string) *ConnectionURLBuilder {
	cb.username(username)
	cb.lines = append(cb.lines, ":")
	cb.password(password)
	return cb
}

func (cb *ConnectionURLBuilder) username(username string) *ConnectionURLBuilder {
	cb.lines = append(cb.lines, username)
	return cb
}

func (cb *ConnectionURLBuilder) password(password string) *ConnectionURLBuilder {
	cb.lines = append(cb.lines, password)
	return cb
}

func (cb *ConnectionURLBuilder) Host(host string) *ConnectionURLBuilder {
	cb.lines = append(cb.lines, "@")
	cb.lines = append(cb.lines, host)
	return cb
}

func (cb *ConnectionURLBuilder) App(appName string) *ConnectionURLBuilder {
	cb.lines = append(cb.lines, "/?appName=")
	cb.lines = append(cb.lines, appName)
	return cb
}

func (cb *ConnectionURLBuilder) Build() string {
	return strings.Join(cb.lines, "")
}

type MongoStoryRepositoryConfig struct {
	ConnectionURL string
}

func NewMongoStoryRepositoryConfig(mongoUsername string, mongoPassword string, host string, mongoApp string, env string) *MongoStoryRepositoryConfig {
	// mongodb+srv://<username>:<password>@prod.skbzy7n.mongodb.net/?appName=prod
	urlBuilder := ConnectionURLBuilder{}
	connectionURL := urlBuilder.Method(env).Credentials(mongoUsername, mongoPassword).Host(host).App(mongoApp).Build()
	// connectionURL := fmt.Sprintf("mongodb+srv://%s:%s@%s/?appName=%s", mongoUsername, mongoPassword, host, mongoApp)
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

// Get gets the story by the the ID
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

// GetBestStory Will be used by the creator service to emit an event to the content generation service with a story ID
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

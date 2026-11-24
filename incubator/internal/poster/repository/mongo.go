// Package repository holds the poster service's record of which stories it has posted.
package repository

import (
	"content-automation-engine/internal/mongodb"
	"content-automation-engine/internal/poster/api"
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// PostsCollection is the collection the poster service owns.
const PostsCollection = "posted_stories"

const (
	statusPosting = "posting"
	statusPosted  = "posted"
)

var _ api.PostRepository = (*MongoPostRepository)(nil)

type MongoPostRepository struct {
	db *mongo.Database
}

func NewMongoPostRepository(db *mongo.Database) *MongoPostRepository {
	return &MongoPostRepository{db: db}
}

type postDocument struct {
	StoryID    string     `bson:"story_id"`
	Platform   string     `bson:"platform"`
	Status     string     `bson:"status"`
	ReservedAt time.Time  `bson:"reserved_at"`
	PostedAt   *time.Time `bson:"posted_at,omitempty"`
}

func (mr *MongoPostRepository) posts() *mongo.Collection {
	return mr.db.Collection(PostsCollection)
}

// EnsureIndexes creates the unique index that makes duplicate posts impossible.
// It is safe to call on every start.
func (mr *MongoPostRepository) EnsureIndexes(ctx context.Context) error {
	_, err := mr.posts().Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "story_id", Value: 1}},
		Options: options.Index().SetName("unique_story_id").SetUnique(true),
	})
	if err != nil {
		return fmt.Errorf("creating post indexes: %w", err)
	}
	return nil
}

// Reserve inserts the story's record. The unique index on story_id means the insert only
// succeeds for the first caller; everyone else gets a duplicate-key error, reported as false.
func (mr *MongoPostRepository) Reserve(ctx context.Context, storyID string, platform string, at time.Time) (bool, error) {
	_, err := mr.posts().InsertOne(ctx, postDocument{
		StoryID:    storyID,
		Platform:   platform,
		Status:     statusPosting,
		ReservedAt: at,
	})
	if mongo.IsDuplicateKeyError(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("reserving story: %w", err)
	}
	return true, nil
}

func (mr *MongoPostRepository) MarkPosted(ctx context.Context, storyID string, at time.Time) error {
	result, err := mr.posts().UpdateOne(ctx,
		bson.D{{Key: "story_id", Value: storyID}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "status", Value: statusPosted}, {Key: "posted_at", Value: at}}}},
	)
	if err != nil {
		return fmt.Errorf("marking story posted: %w", err)
	}
	if result.MatchedCount == 0 {
		return fmt.Errorf("marking story posted: story %s was never reserved", storyID)
	}
	return nil
}

// Release only removes a reservation that has not been completed, so a posted story can
// never be un-recorded.
func (mr *MongoPostRepository) Release(ctx context.Context, storyID string) error {
	_, err := mr.posts().DeleteOne(ctx, bson.D{
		{Key: "story_id", Value: storyID},
		{Key: "status", Value: statusPosting},
	})
	if err != nil {
		return fmt.Errorf("releasing story: %w", err)
	}
	return nil
}

func (mr *MongoPostRepository) StoryIDs(ctx context.Context) ([]string, error) {
	cursor, err := mr.posts().Find(ctx, bson.D{}, options.Find().SetProjection(bson.D{{Key: "story_id", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("listing posted stories: %w", err)
	}

	var docs []postDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("listing posted stories: %w", err)
	}

	ids := make([]string, 0, len(docs))
	for _, doc := range docs {
		ids = append(ids, doc.StoryID)
	}
	return ids, nil
}

// Healthy performs a health check on the repository
func (mr *MongoPostRepository) Healthy(ctx context.Context) bool {
	return mongodb.Healthy(ctx, mr.db)
}

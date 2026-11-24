// Package repository holds the content service's story storage.
package repository

import (
	"content-automation-engine/internal/content/api"
	"content-automation-engine/internal/mongodb"
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// StoriesCollection is the collection the content service owns.
const StoriesCollection = "stories"

var _ api.StoryRepository = (*MongoStoryRepository)(nil)

type MongoStoryRepository struct {
	db *mongo.Database
}

func NewMongoStoryRepository(db *mongo.Database) *MongoStoryRepository {
	return &MongoStoryRepository{db: db}
}

// storyDocument is how a story is stored. Stories saved by earlier versions only have the
// title, body and nsfw fields, so everything else must be treated as optional.
type storyDocument struct {
	ID        bson.ObjectID  `bson:"_id,omitempty"`
	Source    string         `bson:"source,omitempty"`
	SourceID  string         `bson:"source_id,omitempty"`
	Subreddit string         `bson:"subreddit,omitempty"`
	Title     string         `bson:"title"`
	Body      string         `bson:"body"`
	URL       string         `bson:"url,omitempty"`
	CreatedAt time.Time      `bson:"created_at,omitempty"`
	Claim     *claimDocument `bson:"claim,omitempty"`
	Attempts  int            `bson:"attempts,omitempty"`
}

type claimDocument struct {
	Holder    string    `bson:"holder"`
	ExpiresAt time.Time `bson:"expires_at"`
}

func (d storyDocument) toStory() api.Story {
	return api.Story{
		ID:        d.ID.Hex(),
		Source:    d.Source,
		SourceID:  d.SourceID,
		Subreddit: d.Subreddit,
		Title:     d.Title,
		Body:      d.Body,
		URL:       d.URL,
		CreatedAt: d.CreatedAt,
	}
}

func (mr *MongoStoryRepository) stories() *mongo.Collection {
	return mr.db.Collection(StoriesCollection)
}

// EnsureIndexes creates the indexes the repository relies on. It is safe to call on every start.
func (mr *MongoStoryRepository) EnsureIndexes(ctx context.Context) error {
	_, err := mr.stories().Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			// A source's post is only ever stored once. The index is partial because stories
			// saved by earlier versions have no source_id.
			Keys: bson.D{{Key: "source", Value: 1}, {Key: "source_id", Value: 1}},
			Options: options.Index().
				SetName("unique_source_id").
				SetUnique(true).
				SetPartialFilterExpression(bson.D{{Key: "source_id", Value: bson.D{{Key: "$exists", Value: true}}}}),
		},
		{
			Keys:    bson.D{{Key: "claim.expires_at", Value: 1}},
			Options: options.Index().SetName("claim_expiry"),
		},
	})
	if err != nil {
		return fmt.Errorf("creating story indexes: %w", err)
	}
	return nil
}

func (mr *MongoStoryRepository) Save(ctx context.Context, story api.Story) (string, bool, error) {
	doc := storyDocument{
		Source:    story.Source,
		SourceID:  story.SourceID,
		Subreddit: story.Subreddit,
		Title:     story.Title,
		Body:      story.Body,
		URL:       story.URL,
		CreatedAt: story.CreatedAt,
	}

	result, err := mr.stories().InsertOne(ctx, doc)
	if mongo.IsDuplicateKeyError(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("saving story: %w", err)
	}

	id, ok := result.InsertedID.(bson.ObjectID)
	if !ok {
		return "", false, fmt.Errorf("saving story: unexpected id type %T", result.InsertedID)
	}
	return id.Hex(), true, nil
}

func (mr *MongoStoryRepository) GetByID(ctx context.Context, id string) (api.Story, error) {
	objectID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return api.Story{}, fmt.Errorf("%w: invalid id %q", api.ErrStoryNotFound, id)
	}

	var doc storyDocument
	err = mr.stories().FindOne(ctx, bson.D{{Key: "_id", Value: objectID}}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return api.Story{}, api.ErrStoryNotFound
	}
	if err != nil {
		return api.Story{}, fmt.Errorf("getting story: %w", err)
	}
	return doc.toStory(), nil
}

// ClaimNext finds one eligible story and claims it in a single atomic operation
// (FindOneAndUpdate), so two callers can never be handed the same story.
func (mr *MongoStoryRepository) ClaimNext(ctx context.Context, opts api.ClaimOptions) (api.Story, error) {
	filter, err := ClaimFilter(opts)
	if err != nil {
		return api.Story{}, err
	}

	update := bson.D{
		{Key: "$set", Value: bson.D{{Key: "claim", Value: claimDocument{
			Holder:    opts.Holder,
			ExpiresAt: opts.Now.Add(opts.TTL),
		}}}},
		{Key: "$inc", Value: bson.D{{Key: "attempts", Value: 1}}},
	}
	// TODO: rank stories (e.g. by score or topic) once stored stories carry that data.
	// For now the newest eligible story is considered the "best" one.
	findOpts := options.FindOneAndUpdate().
		SetSort(bson.D{{Key: "_id", Value: -1}}).
		SetReturnDocument(options.After)

	var doc storyDocument
	err = mr.stories().FindOneAndUpdate(ctx, filter, update, findOpts).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return api.Story{}, api.ErrNoStoryAvailable
	}
	if err != nil {
		return api.Story{}, fmt.Errorf("claiming story: %w", err)
	}
	return doc.toStory(), nil
}

// ClaimFilter builds the query that decides which stories may be claimed.
func ClaimFilter(opts api.ClaimOptions) (bson.D, error) {
	conditions := bson.A{
		// Not claimed, or the claim has run out.
		bson.D{{Key: "$or", Value: bson.A{
			bson.D{{Key: "claim", Value: bson.D{{Key: "$exists", Value: false}}}},
			bson.D{{Key: "claim.expires_at", Value: bson.D{{Key: "$lt", Value: opts.Now}}}},
		}}},
		bson.D{{Key: "nsfw", Value: bson.D{{Key: "$ne", Value: true}}}},
	}

	if opts.MaxAttempts > 0 {
		conditions = append(conditions, bson.D{{Key: "attempts", Value: bson.D{
			{Key: "$not", Value: bson.D{{Key: "$gte", Value: opts.MaxAttempts}}},
		}}})
	}

	if len(opts.ExcludeIDs) > 0 {
		excluded := make(bson.A, 0, len(opts.ExcludeIDs))
		for _, id := range opts.ExcludeIDs {
			objectID, err := bson.ObjectIDFromHex(id)
			if err != nil {
				return nil, fmt.Errorf("invalid excluded story id %q: %w", id, err)
			}
			excluded = append(excluded, objectID)
		}
		conditions = append(conditions, bson.D{{Key: "_id", Value: bson.D{{Key: "$nin", Value: excluded}}}})
	}

	bodyLength := bson.D{{Key: "$strLenCP", Value: bson.D{{Key: "$ifNull", Value: bson.A{"$body", ""}}}}}
	if opts.MinBodyChars > 0 {
		conditions = append(conditions, bson.D{{Key: "$expr", Value: bson.D{
			{Key: "$gte", Value: bson.A{bodyLength, opts.MinBodyChars}},
		}}})
	}
	if opts.MaxBodyChars > 0 {
		conditions = append(conditions, bson.D{{Key: "$expr", Value: bson.D{
			{Key: "$lte", Value: bson.A{bodyLength, opts.MaxBodyChars}},
		}}})
	}

	return bson.D{{Key: "$and", Value: conditions}}, nil
}

// Performs a health check on the StoryRepository
func (mr *MongoStoryRepository) Healthy(ctx context.Context) bool {
	return mongodb.Healthy(ctx, mr.db)
}

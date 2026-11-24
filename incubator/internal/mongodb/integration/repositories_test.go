//go:build integration

// Package integration checks the MongoDB repositories against a real MongoDB server.
//
// These tests only run with `go test -tags integration` and need MONGO_TEST_URI to point at
// a disposable server, for example:
//
//	docker run --rm -d -p 27018:27017 mongo:8.3.2
//	MONGO_TEST_URI=mongodb://localhost:27018 go test -tags integration ./internal/mongodb/integration/
//
// Each test uses its own freshly created database and drops it afterwards.
package integration

import (
	contentapi "content-automation-engine/internal/content/api"
	contentrepository "content-automation-engine/internal/content/repository"
	"content-automation-engine/internal/mongodb"
	posterrepository "content-automation-engine/internal/poster/repository"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func testDatabase(t *testing.T) *mongo.Database {
	t.Helper()
	uri := os.Getenv("MONGO_TEST_URI")
	if uri == "" {
		t.Skip("MONGO_TEST_URI is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := mongodb.Connect(ctx, uri)
	require.NoError(t, err)

	db := client.Database(fmt.Sprintf("engine_test_%d", time.Now().UnixNano()))
	t.Cleanup(func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	})
	return db
}

var now = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

func claim(excluded ...string) contentapi.ClaimOptions {
	return contentapi.ClaimOptions{
		Holder: "test", Now: now, TTL: 30 * time.Minute, MaxAttempts: 3,
		MinBodyChars: 10, MaxBodyChars: 100, ExcludeIDs: excluded,
	}
}

const body = "A body that is long enough to claim."

func TestStoriesAreSavedOncePerSourcePost(t *testing.T) {
	db := testDatabase(t)
	ctx := context.Background()
	repo := contentrepository.NewMongoStoryRepository(db)
	require.NoError(t, repo.EnsureIndexes(ctx))
	require.NoError(t, repo.EnsureIndexes(ctx), "creating the indexes twice is harmless")

	story := contentapi.Story{Source: "reddit", SourceID: "t3_abc", Subreddit: "tifu", Title: "Title", Body: body, URL: "https://example.com", CreatedAt: now}
	id, created, err := repo.Save(ctx, story)
	require.NoError(t, err)
	assert.True(t, created)

	_, created, err = repo.Save(ctx, story)
	require.NoError(t, err)
	assert.False(t, created, "the same Reddit post is not stored twice")

	other := story
	other.SourceID = "t3_xyz"
	_, created, err = repo.Save(ctx, other)
	require.NoError(t, err)
	assert.True(t, created)

	got, err := repo.GetByID(ctx, id)
	require.NoError(t, err)
	story.ID = id
	assert.Equal(t, story, got)
	assert.True(t, repo.Healthy(ctx))
}

func TestGetByIDReportsMissingStories(t *testing.T) {
	repo := contentrepository.NewMongoStoryRepository(testDatabase(t))

	_, err := repo.GetByID(context.Background(), bson.NewObjectID().Hex())
	require.ErrorIs(t, err, contentapi.ErrStoryNotFound)

	_, err = repo.GetByID(context.Background(), "not-an-id")
	require.ErrorIs(t, err, contentapi.ErrStoryNotFound)
}

func TestStoriesSavedByEarlierVersionsCanStillBeSavedAndClaimed(t *testing.T) {
	db := testDatabase(t)
	ctx := context.Background()
	// Earlier versions stored only these three fields, with no source_id.
	legacy := []any{
		bson.D{{Key: "title", Value: "Old one"}, {Key: "body", Value: body}, {Key: "nsfw", Value: false}},
		bson.D{{Key: "title", Value: "Old two"}, {Key: "body", Value: body}, {Key: "nsfw", Value: false}},
	}
	_, err := db.Collection(contentrepository.StoriesCollection).InsertMany(ctx, legacy)
	require.NoError(t, err)

	repo := contentrepository.NewMongoStoryRepository(db)
	require.NoError(t, repo.EnsureIndexes(ctx), "the unique index must tolerate stories that have no source_id")

	first, err := repo.ClaimNext(ctx, claim())
	require.NoError(t, err)
	second, err := repo.ClaimNext(ctx, claim())
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"Old one", "Old two"}, []string{first.Title, second.Title})
}

func TestClaimNextAppliesEveryRule(t *testing.T) {
	db := testDatabase(t)
	ctx := context.Background()
	repo := contentrepository.NewMongoStoryRepository(db)
	require.NoError(t, repo.EnsureIndexes(ctx))

	save := func(sourceID string, storyBody string) string {
		id, _, err := repo.Save(ctx, contentapi.Story{Source: "reddit", SourceID: sourceID, Title: sourceID, Body: storyBody})
		require.NoError(t, err)
		return id
	}
	oldest := save("oldest", body)
	posted := save("posted", body)
	save("too-short", "tiny")
	save("too-long", strings.Repeat("x", 101))
	_, err := db.Collection(contentrepository.StoriesCollection).InsertOne(ctx,
		bson.D{{Key: "title", Value: "nsfw"}, {Key: "body", Value: body}, {Key: "nsfw", Value: true}})
	require.NoError(t, err)
	newest := save("newest", body)

	// Newest eligible story first.
	story, err := repo.ClaimNext(ctx, claim(posted))
	require.NoError(t, err)
	assert.Equal(t, newest, story.ID)

	// The claimed one is skipped; "posted" is excluded; the rest are unsuitable.
	story, err = repo.ClaimNext(ctx, claim(posted))
	require.NoError(t, err)
	assert.Equal(t, oldest, story.ID)

	_, err = repo.ClaimNext(ctx, claim(posted))
	require.ErrorIs(t, err, contentapi.ErrNoStoryAvailable)

	// Without the exclusion, the remaining eligible story is the "posted" one.
	story, err = repo.ClaimNext(ctx, claim())
	require.NoError(t, err)
	assert.Equal(t, posted, story.ID)
}

func TestClaimsExpireAndAttemptsRunOut(t *testing.T) {
	db := testDatabase(t)
	ctx := context.Background()
	repo := contentrepository.NewMongoStoryRepository(db)
	id, _, err := repo.Save(ctx, contentapi.Story{Source: "reddit", SourceID: "only", Title: "Only", Body: body})
	require.NoError(t, err)

	opts := claim()
	for attempt := 1; attempt <= opts.MaxAttempts; attempt++ {
		story, err := repo.ClaimNext(ctx, opts)
		require.NoError(t, err, "attempt %d", attempt)
		assert.Equal(t, id, story.ID)

		_, err = repo.ClaimNext(ctx, opts)
		require.ErrorIs(t, err, contentapi.ErrNoStoryAvailable, "held while the claim is live")

		opts.Now = opts.Now.Add(opts.TTL + time.Second)
	}

	_, err = repo.ClaimNext(ctx, opts)
	require.ErrorIs(t, err, contentapi.ErrNoStoryAvailable, "given up on after the last attempt")
}

func TestConcurrentClaimsNeverReturnTheSameStory(t *testing.T) {
	db := testDatabase(t)
	ctx := context.Background()
	repo := contentrepository.NewMongoStoryRepository(db)
	const stories, callers = 10, 40
	for i := range stories {
		_, _, err := repo.Save(ctx, contentapi.Story{Source: "reddit", SourceID: fmt.Sprint(i), Title: "Story", Body: body})
		require.NoError(t, err)
	}

	var mu sync.Mutex
	claimed := make(map[string]int)
	var wg sync.WaitGroup
	for range callers {
		wg.Go(func() {
			story, err := repo.ClaimNext(ctx, claim())
			if err != nil {
				return
			}
			mu.Lock()
			claimed[story.ID]++
			mu.Unlock()
		})
	}
	wg.Wait()

	assert.Len(t, claimed, stories)
	for id, count := range claimed {
		assert.Equal(t, 1, count, "story %s was claimed %d times", id, count)
	}
}

func TestPostsAreReservedOnce(t *testing.T) {
	db := testDatabase(t)
	ctx := context.Background()
	repo := posterrepository.NewMongoPostRepository(db)
	require.NoError(t, repo.EnsureIndexes(ctx))
	require.NoError(t, repo.EnsureIndexes(ctx))

	// Many callers race to reserve the same story: exactly one wins.
	var mu sync.Mutex
	wins := 0
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			reserved, err := repo.Reserve(ctx, "story-1", "tiktok", now)
			if err == nil && reserved {
				mu.Lock()
				wins++
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	assert.Equal(t, 1, wins)

	ids, err := repo.StoryIDs(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{"story-1"}, ids, "a story being posted is already listed")
	assert.True(t, repo.Healthy(ctx))
}

func TestReleaseOnlyUndoesUnfinishedPosts(t *testing.T) {
	db := testDatabase(t)
	ctx := context.Background()
	repo := posterrepository.NewMongoPostRepository(db)
	require.NoError(t, repo.EnsureIndexes(ctx))

	_, err := repo.Reserve(ctx, "failed", "tiktok", now)
	require.NoError(t, err)
	require.NoError(t, repo.Release(ctx, "failed"))
	reserved, err := repo.Reserve(ctx, "failed", "tiktok", now)
	require.NoError(t, err)
	assert.True(t, reserved, "a released story can be reserved again")

	_, err = repo.Reserve(ctx, "done", "tiktok", now)
	require.NoError(t, err)
	require.NoError(t, repo.MarkPosted(ctx, "done", now))
	require.NoError(t, repo.Release(ctx, "done"))
	reserved, err = repo.Reserve(ctx, "done", "tiktok", now)
	require.NoError(t, err)
	assert.False(t, reserved, "a posted story can never be released and reposted")

	require.Error(t, repo.MarkPosted(ctx, "never-reserved", now))
}

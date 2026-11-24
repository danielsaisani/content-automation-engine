// Package testutil holds in-memory fakes of everything the engine talks to in the outside
// world, so tests never need a real database, Reddit, text-to-speech server, ffmpeg or TikTok.
package testutil

import (
	"content-automation-engine/internal/app/clock"
	contentapi "content-automation-engine/internal/content/api"
	generatorapi "content-automation-engine/internal/generator/api"
	posterapi "content-automation-engine/internal/poster/api"
	"content-automation-engine/internal/service"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// NewDependencies returns service dependencies with a silent logger and a clock tests control.
func NewDependencies() (*service.ServiceDependencies, *clock.MockClock) {
	mockClock := clock.NewMockClock()
	mockClock.SetTime(time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC))
	return &service.ServiceDependencies{
		Clock:  mockClock,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}, mockClock
}

// StoryRepository is an in-memory contentapi.StoryRepository with the same claiming rules
// as the MongoDB one.
type StoryRepository struct {
	mu        sync.Mutex
	nextID    int
	order     []string
	stories   map[string]contentapi.Story
	claims    map[string]time.Time
	attempts  map[string]int
	SaveErr   error
	Unhealthy bool
}

var _ contentapi.StoryRepository = (*StoryRepository)(nil)

func NewStoryRepository() *StoryRepository {
	return &StoryRepository{
		stories:  make(map[string]contentapi.Story),
		claims:   make(map[string]time.Time),
		attempts: make(map[string]int),
	}
}

func (r *StoryRepository) Save(ctx context.Context, story contentapi.Story) (string, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.SaveErr != nil {
		return "", false, r.SaveErr
	}
	if story.SourceID != "" {
		for _, existing := range r.stories {
			if existing.Source == story.Source && existing.SourceID == story.SourceID {
				return "", false, nil
			}
		}
	}

	r.nextID++
	story.ID = fmt.Sprintf("story-%d", r.nextID)
	r.stories[story.ID] = story
	r.order = append(r.order, story.ID)
	return story.ID, true, nil
}

func (r *StoryRepository) GetByID(ctx context.Context, id string) (contentapi.Story, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	story, ok := r.stories[id]
	if !ok {
		return contentapi.Story{}, contentapi.ErrStoryNotFound
	}
	return story, nil
}

func (r *StoryRepository) ClaimNext(ctx context.Context, opts contentapi.ClaimOptions) (contentapi.Story, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Newest first, like the MongoDB repository.
	for i := len(r.order) - 1; i >= 0; i-- {
		id := r.order[i]
		story := r.stories[id]
		length := utf8.RuneCountInString(story.Body)

		if expiry, claimed := r.claims[id]; claimed && !expiry.Before(opts.Now) {
			continue
		}
		if opts.MaxAttempts > 0 && r.attempts[id] >= opts.MaxAttempts {
			continue
		}
		if slices.Contains(opts.ExcludeIDs, id) {
			continue
		}
		if opts.MinBodyChars > 0 && length < opts.MinBodyChars {
			continue
		}
		if opts.MaxBodyChars > 0 && length > opts.MaxBodyChars {
			continue
		}

		r.claims[id] = opts.Now.Add(opts.TTL)
		r.attempts[id]++
		return story, nil
	}
	return contentapi.Story{}, contentapi.ErrNoStoryAvailable
}

// SetSaveErr makes every later Save fail with err; nil makes Save work again.
func (r *StoryRepository) SetSaveErr(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.SaveErr = err
}

func (r *StoryRepository) Healthy(ctx context.Context) bool { return !r.Unhealthy }

// Stories returns every stored story, oldest first.
func (r *StoryRepository) Stories() []contentapi.Story {
	r.mu.Lock()
	defer r.mu.Unlock()

	stories := make([]contentapi.Story, 0, len(r.order))
	for _, id := range r.order {
		stories = append(stories, r.stories[id])
	}
	return stories
}

// PostRepository is an in-memory posterapi.PostRepository.
type PostRepository struct {
	mu          sync.Mutex
	status      map[string]string
	ReserveErr  error
	StoryIDsErr error
}

var _ posterapi.PostRepository = (*PostRepository)(nil)

func NewPostRepository() *PostRepository {
	return &PostRepository{status: make(map[string]string)}
}

func (r *PostRepository) Reserve(ctx context.Context, storyID string, platform string, at time.Time) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.ReserveErr != nil {
		return false, r.ReserveErr
	}
	if _, exists := r.status[storyID]; exists {
		return false, nil
	}
	r.status[storyID] = "posting"
	return true, nil
}

func (r *PostRepository) MarkPosted(ctx context.Context, storyID string, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.status[storyID]; !exists {
		return errors.New("story was never reserved")
	}
	r.status[storyID] = "posted"
	return nil
}

func (r *PostRepository) Release(ctx context.Context, storyID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.status[storyID] == "posting" {
		delete(r.status, storyID)
	}
	return nil
}

func (r *PostRepository) StoryIDs(ctx context.Context) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.StoryIDsErr != nil {
		return nil, r.StoryIDsErr
	}
	ids := make([]string, 0, len(r.status))
	for id := range r.status {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids, nil
}

func (r *PostRepository) Healthy(ctx context.Context) bool { return true }

// Status returns "posting", "posted" or "" for a story.
func (r *PostRepository) Status(storyID string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.status[storyID]
}

// Scraper is a contentapi.Scraper that emits whatever a test sends it.
type Scraper struct {
	posts     chan *contentapi.ScrapedContent
	Unhealthy bool
}

var _ contentapi.Scraper = (*Scraper)(nil)

func NewScraper() *Scraper {
	return &Scraper{posts: make(chan *contentapi.ScrapedContent, 100)}
}

// Emit queues a scraped post.
func (s *Scraper) Emit(post *contentapi.ScrapedContent) { s.posts <- post }

func (s *Scraper) Posts() <-chan *contentapi.ScrapedContent { return s.posts }

func (s *Scraper) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (s *Scraper) Healthy(ctx context.Context) bool { return !s.Unhealthy }

// Narrator is a generatorapi.Narrator that writes a small placeholder audio file.
type Narrator struct {
	mu    sync.Mutex
	texts []string
	// Err, when set, makes every narration fail.
	Err error
	// FailIfContains, when set, makes narrating any text containing it fail.
	FailIfContains string
}

var _ generatorapi.Narrator = (*Narrator)(nil)

func (n *Narrator) Narrate(ctx context.Context, text string, outputPath string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.Err != nil {
		return n.Err
	}
	if n.FailIfContains != "" && strings.Contains(text, n.FailIfContains) {
		return errors.New("narrator failed")
	}
	n.texts = append(n.texts, text)
	return os.WriteFile(outputPath, []byte("fake audio"), 0o644)
}

// Texts returns everything the narrator was asked to read.
func (n *Narrator) Texts() []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return slices.Clone(n.texts)
}

// Renderer is a generatorapi.Renderer that writes a small placeholder video file.
type Renderer struct {
	mu       sync.Mutex
	requests []generatorapi.RenderRequest
	// Err, when set, makes every render fail.
	Err error
	// FailIfContains, when set, makes rendering any narration containing it fail.
	FailIfContains string
}

var _ generatorapi.Renderer = (*Renderer)(nil)

func (r *Renderer) Render(ctx context.Context, request generatorapi.RenderRequest) (time.Duration, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.Err != nil {
		return 0, r.Err
	}
	if r.FailIfContains != "" && strings.Contains(request.Narration, r.FailIfContains) {
		return 0, errors.New("renderer failed")
	}
	r.requests = append(r.requests, request)
	return 42 * time.Second, os.WriteFile(request.OutputPath, []byte("fake video"), 0o644)
}

// Requests returns every render request received.
func (r *Renderer) Requests() []generatorapi.RenderRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.requests)
}

// Platform is a posterapi.Platform that records what it was asked to publish.
type Platform struct {
	mu           sync.Mutex
	publications []posterapi.Publication
	// VideoExisted records, per publication, whether the video file was on disk at the time.
	VideoExisted []bool
	Err          error
}

var _ posterapi.Platform = (*Platform)(nil)

func (p *Platform) Name() string { return "fake" }

func (p *Platform) Publish(ctx context.Context, publication posterapi.Publication) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Err != nil {
		return p.Err
	}
	_, statErr := os.Stat(publication.VideoPath)
	p.publications = append(p.publications, publication)
	p.VideoExisted = append(p.VideoExisted, statErr == nil)
	return nil
}

// Publications returns everything published so far.
func (p *Platform) Publications() []posterapi.Publication {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.publications)
}

// PostedStories is a posterapi.PostedStories with a fixed answer.
type PostedStories struct {
	IDs []string
	Err error
}

var _ posterapi.PostedStories = (*PostedStories)(nil)

func (p *PostedStories) PostedStoryIDs(ctx context.Context) ([]string, error) {
	return p.IDs, p.Err
}

// ContentService is a contentapi.ContentService serving stories from a map.
type ContentService struct {
	Stories map[string]contentapi.Story
}

var _ contentapi.ContentService = (*ContentService)(nil)

func (c *ContentService) Run(ctx context.Context) error    { <-ctx.Done(); return nil }
func (c *ContentService) Healthy(ctx context.Context) bool { return true }
func (c *ContentService) Name(ctx context.Context) string  { return "Content" }

func (c *ContentService) GetStoryByID(ctx context.Context, id string) (contentapi.Story, error) {
	story, ok := c.Stories[id]
	if !ok {
		return contentapi.Story{}, contentapi.ErrStoryNotFound
	}
	return story, nil
}

func (c *ContentService) ClaimStory(ctx context.Context, holder string) (contentapi.Story, error) {
	return contentapi.Story{}, contentapi.ErrNoStoryAvailable
}

// GeneratorService is a generatorapi.GeneratorService serving videos from a map.
type GeneratorService struct {
	mu        sync.Mutex
	Videos    map[string]generatorapi.Video
	discarded []string
}

var _ generatorapi.GeneratorService = (*GeneratorService)(nil)

func (g *GeneratorService) Run(ctx context.Context) error    { <-ctx.Done(); return nil }
func (g *GeneratorService) Healthy(ctx context.Context) bool { return true }
func (g *GeneratorService) Name(ctx context.Context) string  { return "Generator" }

func (g *GeneratorService) GetVideoByID(ctx context.Context, id string) (generatorapi.Video, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	video, ok := g.Videos[id]
	if !ok {
		return generatorapi.Video{}, generatorapi.ErrVideoNotFound
	}
	return video, nil
}

func (g *GeneratorService) DiscardVideo(ctx context.Context, id string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.discarded = append(g.discarded, id)
	return nil
}

// Discarded returns the IDs of every discarded video.
func (g *GeneratorService) Discarded() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return slices.Clone(g.discarded)
}

// Eventually polls check until it returns true or the timeout passes, and reports the result.
func Eventually(timeout time.Duration, check func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if check() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return check()
}

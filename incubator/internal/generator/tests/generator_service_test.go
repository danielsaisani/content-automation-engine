package tests

import (
	contentapi "content-automation-engine/internal/content/api"
	"content-automation-engine/internal/events"
	"content-automation-engine/internal/generator"
	generatorapi "content-automation-engine/internal/generator/api"
	"content-automation-engine/internal/testutil"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type generatorHarness struct {
	service  *generator.GeneratorService
	content  *testutil.ContentService
	narrator *testutil.Narrator
	renderer *testutil.Renderer
	selected chan events.StorySelected
	ready    chan events.VideoReady
	dir      string
}

func newGeneratorHarness(t *testing.T) *generatorHarness {
	t.Helper()
	deps, _ := testutil.NewDependencies()
	h := &generatorHarness{
		content: &testutil.ContentService{Stories: map[string]contentapi.Story{
			"story-1": {ID: "story-1", Title: "**TIFU** by testing", Body: "It went [well](https://example.com). Really well."},
		}},
		narrator: &testutil.Narrator{},
		renderer: &testutil.Renderer{},
		selected: make(chan events.StorySelected, 10),
		ready:    make(chan events.VideoReady, 10),
		dir:      t.TempDir(),
	}
	cfg := &generator.Config{OutputDir: h.dir, MaxNarrationChars: 2500}
	h.service = generator.NewGeneratorService(deps, cfg, h.selected, h.ready, h.content, h.narrator, h.renderer)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- h.service.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		require.NoError(t, <-done)
	})
	return h
}

func (h *generatorHarness) awaitVideo(t *testing.T) events.VideoReady {
	t.Helper()
	select {
	case ready := <-h.ready:
		return ready
	case <-time.After(2 * time.Second):
		t.Fatal("no VideoReady event was emitted")
		return events.VideoReady{}
	}
}

func (h *generatorHarness) leftoverFolders(t *testing.T) int {
	t.Helper()
	entries, err := os.ReadDir(h.dir)
	require.NoError(t, err)
	return len(entries)
}

func TestSelectedStoryBecomesAVideo(t *testing.T) {
	h := newGeneratorHarness(t)

	h.selected <- events.StorySelected{StoryID: "story-1"}
	ready := h.awaitVideo(t)

	assert.Equal(t, "story-1", ready.StoryID)
	video, err := h.service.GetVideoByID(context.Background(), ready.VideoID)
	require.NoError(t, err)
	assert.Equal(t, "story-1", video.StoryID)
	assert.Equal(t, "TIFU by testing", video.Title, "the title is cleaned for captions")
	assert.Equal(t, 42*time.Second, video.Duration)
	assert.True(t, filepath.IsAbs(video.Path))
	assert.FileExists(t, video.Path)

	// The narration is the cleaned title followed by the cleaned body.
	assert.Equal(t, []string{"TIFU by testing. It went well. Really well."}, h.narrator.Texts())
	require.Len(t, h.renderer.Requests(), 1)
	assert.Equal(t, "TIFU by testing. It went well. Really well.", h.renderer.Requests()[0].Narration)
}

func TestDiscardVideoDeletesItsFiles(t *testing.T) {
	h := newGeneratorHarness(t)
	h.selected <- events.StorySelected{StoryID: "story-1"}
	ready := h.awaitVideo(t)
	video, err := h.service.GetVideoByID(context.Background(), ready.VideoID)
	require.NoError(t, err)

	require.NoError(t, h.service.DiscardVideo(context.Background(), ready.VideoID))

	assert.NoFileExists(t, video.Path)
	assert.Equal(t, 0, h.leftoverFolders(t))
	_, err = h.service.GetVideoByID(context.Background(), ready.VideoID)
	require.ErrorIs(t, err, generatorapi.ErrVideoNotFound)
	require.ErrorIs(t, h.service.DiscardVideo(context.Background(), ready.VideoID), generatorapi.ErrVideoNotFound)
}

func TestFailuresEmitNoEventAndLeaveNoFiles(t *testing.T) {
	// Each case makes one story fail at a different step.
	cases := map[string]func(h *generatorHarness) string{
		"unknown story": func(h *generatorHarness) string { return "missing" },
		"narrator fails": func(h *generatorHarness) string {
			h.narrator.FailIfContains = "by testing"
			return "story-1"
		},
		"renderer fails": func(h *generatorHarness) string {
			h.renderer.FailIfContains = "by testing"
			return "story-1"
		},
		"nothing to read": func(h *generatorHarness) string {
			h.content.Stories["empty"] = contentapi.Story{ID: "empty", Title: "  ", Body: "**"}
			return "empty"
		},
	}

	for name, arrange := range cases {
		t.Run(name, func(t *testing.T) {
			h := newGeneratorHarness(t)
			h.content.Stories["good"] = contentapi.Story{ID: "good", Title: "Good", Body: "Fine."}
			failing := arrange(h)

			h.selected <- events.StorySelected{StoryID: failing}
			// A later, good story still gets through: the service keeps running after a failure.
			h.selected <- events.StorySelected{StoryID: "good"}

			ready := h.awaitVideo(t)
			assert.Equal(t, "good", ready.StoryID, "the failed story emits no event")
			assert.Equal(t, 1, h.leftoverFolders(t), "only the good video's folder remains")
		})
	}
}

func TestNarrationIsCappedAtTheConfiguredLength(t *testing.T) {
	story := contentapi.Story{Title: "Title", Body: strings.Repeat("A sentence. ", 100)}

	narration := generator.Narration(story, 60)

	assert.LessOrEqual(t, len(narration), 60)
	assert.True(t, strings.HasPrefix(narration, "Title. A sentence."))
	assert.True(t, strings.HasSuffix(narration, "."), "the cut lands on the end of a sentence")
}

func TestNarrationKeepsTheTitlesOwnPunctuation(t *testing.T) {
	assert.Equal(t, "AITA? Yes.", generator.Narration(contentapi.Story{Title: "AITA?", Body: "Yes."}, 0))
	assert.Equal(t, "Only a body.", generator.Narration(contentapi.Story{Body: "Only a body."}, 0))
	assert.Equal(t, "Only a title.", generator.Narration(contentapi.Story{Title: "Only a title"}, 0))
}

func TestLoadGeneratorConfig(t *testing.T) {
	defaults, err := generator.LoadConfig()
	require.NoError(t, err)
	assert.NotEmpty(t, defaults.OutputDir, "a default output folder is chosen")
	assert.Equal(t, 2500, defaults.MaxNarrationChars)

	t.Setenv("GENERATOR_OUTPUT_DIR", "/data/videos")
	cfg, err := generator.LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, "/data/videos", cfg.OutputDir)
}

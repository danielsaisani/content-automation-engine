package api

import (
	sharedapi "content-automation-engine/internal/api"
	"context"
	"errors"
	"time"
)

// ErrVideoNotFound means no video has the requested ID.
var ErrVideoNotFound = errors.New("video not found")

// Video is a generated video for one story.
type Video struct {
	ID      string
	StoryID string
	// Title is the story's title, for use in captions.
	Title string
	// Path is the absolute path of the video file.
	Path     string
	Duration time.Duration
}

// GeneratorService is the contract other services use to talk to the generator service.
type GeneratorService interface {
	sharedapi.Service
	// GetVideoByID returns the video with the given ID, or ErrVideoNotFound.
	GetVideoByID(ctx context.Context, id string) (Video, error)
	// DiscardVideo deletes a video's files once nobody needs them any more.
	DiscardVideo(ctx context.Context, id string) error
}

// Narrator turns text into speech.
type Narrator interface {
	// Narrate writes an audio file reading text aloud to outputPath.
	Narrate(ctx context.Context, text string, outputPath string) error
}

// RenderRequest describes one video to render.
type RenderRequest struct {
	// AudioPath is the narration audio file.
	AudioPath string
	// Narration is the text being read, shown on screen as captions.
	Narration string
	// OutputPath is where the video file is written.
	OutputPath string
}

// Renderer turns narration audio and text into a video.
type Renderer interface {
	// Render writes the video and returns its duration.
	Render(ctx context.Context, request RenderRequest) (time.Duration, error)
}

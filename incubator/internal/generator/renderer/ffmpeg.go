// Package renderer holds the video renderers the generator can use.
package renderer

import (
	"content-automation-engine/internal/generator/api"
	"content-automation-engine/internal/generator/captions"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/vrischmann/envconfig"
)

// Config is read from environment variables prefixed with GENERATOR_.
type Config struct {
	// Field names here are spelled so envconfig derives GENERATOR_FFMPEG_PATH and
	// GENERATOR_FFPROBE_PATH; "FFmpegPath" would become GENERATOR_F_FMPEG_PATH.
	FfmpegPath  string `envconfig:"default=ffmpeg"`
	FfprobePath string `envconfig:"default=ffprobe"`
	// BackgroundVideo is an optional clip looped behind the captions. Without it the
	// background is a plain colour.
	BackgroundVideo string `envconfig:"optional"`
	// BackgroundColor is used when there is no background video.
	BackgroundColor string `envconfig:"default=0x14171f"`
	CaptionFont     string `envconfig:"default=DejaVu Sans"`
	CaptionFontSize int    `envconfig:"default=84"`
	WordsPerCaption int    `envconfig:"default=5"`
}

// LoadConfig reads the renderer configuration from the environment.
func LoadConfig() (*Config, error) {
	cfg := &Config{}
	if err := envconfig.InitWithPrefix(cfg, "GENERATOR"); err != nil {
		return nil, fmt.Errorf("loading renderer config: %w", err)
	}
	return cfg, nil
}

const (
	videoWidth  = 1080
	videoHeight = 1920
)

// CommandRunner runs an external program in dir and returns its standard output.
// It exists so tests can check the commands without running ffmpeg.
type CommandRunner func(ctx context.Context, dir string, name string, args ...string) ([]byte, error)

// ExecRunner runs the command for real.
func ExecRunner(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return out, fmt.Errorf("%s: %w: %s", name, err, lastLines(stderr.String(), 5))
	}
	return out, nil
}

var _ api.Renderer = (*FFmpegRenderer)(nil)

// FFmpegRenderer renders a vertical video: a background, the narration audio and captions.
type FFmpegRenderer struct {
	cfg *Config
	run CommandRunner
}

func NewFFmpegRenderer(cfg *Config) *FFmpegRenderer {
	return NewFFmpegRendererWithRunner(cfg, ExecRunner)
}

func NewFFmpegRendererWithRunner(cfg *Config, run CommandRunner) *FFmpegRenderer {
	return &FFmpegRenderer{cfg: cfg, run: run}
}

func (r *FFmpegRenderer) Render(ctx context.Context, request api.RenderRequest) (time.Duration, error) {
	duration, err := r.audioDuration(ctx, request.AudioPath)
	if err != nil {
		return 0, err
	}

	// ffmpeg runs inside the output directory and is given the caption file by its bare
	// name, which avoids escaping an arbitrary path inside the filter expression.
	dir := filepath.Dir(request.OutputPath)
	const captionFile = "captions.ass"
	style := captions.Style{Font: r.cfg.CaptionFont, FontSize: r.cfg.CaptionFontSize, Width: videoWidth, Height: videoHeight}
	subtitles := captions.ASS(captions.Split(request.Narration, r.cfg.WordsPerCaption), duration, style)
	if err := os.WriteFile(filepath.Join(dir, captionFile), []byte(subtitles), 0o644); err != nil {
		return 0, fmt.Errorf("rendering: writing captions: %w", err)
	}

	args := r.ffmpegArgs(request, duration, captionFile)
	if _, err := r.run(ctx, dir, r.cfg.FfmpegPath, args...); err != nil {
		return 0, fmt.Errorf("rendering: %w", err)
	}
	return duration, nil
}

func (r *FFmpegRenderer) ffmpegArgs(request api.RenderRequest, duration time.Duration, captionFile string) []string {
	args := []string{"-y", "-hide_banner", "-loglevel", "error"}

	filter := "ass=" + captionFile
	if r.cfg.BackgroundVideo != "" {
		args = append(args, "-stream_loop", "-1", "-i", r.cfg.BackgroundVideo)
		filter = fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d,%s",
			videoWidth, videoHeight, videoWidth, videoHeight, filter)
	} else {
		args = append(args, "-f", "lavfi", "-i",
			fmt.Sprintf("color=c=%s:s=%dx%d:r=30", r.cfg.BackgroundColor, videoWidth, videoHeight))
	}

	return append(args,
		"-i", request.AudioPath,
		"-map", "0:v:0", "-map", "1:a:0",
		"-vf", filter,
		"-t", strconv.FormatFloat(duration.Seconds(), 'f', 3, 64),
		"-c:v", "libx264", "-preset", "veryfast", "-pix_fmt", "yuv420p", "-r", "30",
		"-c:a", "aac", "-b:a", "192k",
		"-movflags", "+faststart",
		request.OutputPath,
	)
}

func (r *FFmpegRenderer) audioDuration(ctx context.Context, audioPath string) (time.Duration, error) {
	out, err := r.run(ctx, "", r.cfg.FfprobePath,
		"-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", audioPath)
	if err != nil {
		return 0, fmt.Errorf("rendering: reading audio duration: %w", err)
	}

	seconds, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil || seconds <= 0 {
		return 0, fmt.Errorf("rendering: unexpected audio duration %q", strings.TrimSpace(string(out)))
	}
	return time.Duration(seconds * float64(time.Second)), nil
}

func lastLines(text string, n int) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, " | ")
}

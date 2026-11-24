package tests

import (
	generatorapi "content-automation-engine/internal/generator/api"
	"content-automation-engine/internal/generator/renderer"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordedCommand struct {
	dir  string
	name string
	args []string
}

// fakeRunner stands in for ffprobe and ffmpeg.
type fakeRunner struct {
	commands  []recordedCommand
	duration  string
	ffmpegErr error
}

func (f *fakeRunner) run(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	f.commands = append(f.commands, recordedCommand{dir: dir, name: name, args: args})
	if name == "ffprobe" {
		return []byte(f.duration + "\n"), nil
	}
	return nil, f.ffmpegErr
}

func rendererConfig() *renderer.Config {
	return &renderer.Config{
		FfmpegPath: "ffmpeg", FfprobePath: "ffprobe", BackgroundColor: "0x14171f",
		CaptionFont: "DejaVu Sans", CaptionFontSize: 84, WordsPerCaption: 3,
	}
}

func TestRenderBuildsAVerticalVideoCommand(t *testing.T) {
	runner := &fakeRunner{duration: "12.500000"}
	dir := t.TempDir()
	request := generatorapi.RenderRequest{
		AudioPath:  filepath.Join(dir, "narration.mp3"),
		Narration:  "One two three four five six.",
		OutputPath: filepath.Join(dir, "video.mp4"),
	}

	duration, err := renderer.NewFFmpegRendererWithRunner(rendererConfig(), runner.run).Render(context.Background(), request)

	require.NoError(t, err)
	assert.Equal(t, 12500*time.Millisecond, duration)
	require.Len(t, runner.commands, 2)

	probe := runner.commands[0]
	assert.Equal(t, "ffprobe", probe.name)
	assert.Equal(t, request.AudioPath, probe.args[len(probe.args)-1])

	ffmpeg := runner.commands[1]
	args := strings.Join(ffmpeg.args, " ")
	assert.Equal(t, "ffmpeg", ffmpeg.name)
	assert.Equal(t, dir, ffmpeg.dir, "ffmpeg runs in the output folder so the caption file needs no escaping")
	assert.Contains(t, args, "color=c=0x14171f:s=1080x1920:r=30")
	assert.Contains(t, args, "-i "+request.AudioPath)
	assert.Contains(t, args, "-vf ass=captions.ass")
	assert.Contains(t, args, "-t 12.500")
	assert.Equal(t, request.OutputPath, ffmpeg.args[len(ffmpeg.args)-1])

	captionFile, err := os.ReadFile(filepath.Join(dir, "captions.ass"))
	require.NoError(t, err)
	assert.Contains(t, string(captionFile), "One two three")
	assert.Contains(t, string(captionFile), "four five six.")
}

func TestRenderLoopsABackgroundVideoWhenConfigured(t *testing.T) {
	runner := &fakeRunner{duration: "5"}
	cfg := rendererConfig()
	cfg.BackgroundVideo = "/assets/gameplay.mp4"
	dir := t.TempDir()

	_, err := renderer.NewFFmpegRendererWithRunner(cfg, runner.run).Render(context.Background(), generatorapi.RenderRequest{
		AudioPath: filepath.Join(dir, "a.mp3"), Narration: "Hello.", OutputPath: filepath.Join(dir, "v.mp4"),
	})

	require.NoError(t, err)
	args := strings.Join(runner.commands[1].args, " ")
	assert.Contains(t, args, "-stream_loop -1 -i /assets/gameplay.mp4")
	assert.Contains(t, args, "scale=1080:1920:force_original_aspect_ratio=increase,crop=1080:1920,ass=captions.ass")
	assert.NotContains(t, args, "lavfi")
}

func TestRenderFailsOnAnUnreadableDuration(t *testing.T) {
	for _, duration := range []string{"", "N/A", "0", "-3"} {
		runner := &fakeRunner{duration: duration}
		dir := t.TempDir()

		_, err := renderer.NewFFmpegRendererWithRunner(rendererConfig(), runner.run).Render(context.Background(), generatorapi.RenderRequest{
			AudioPath: filepath.Join(dir, "a.mp3"), Narration: "Hello.", OutputPath: filepath.Join(dir, "v.mp4"),
		})

		require.Error(t, err, "duration %q", duration)
		assert.Len(t, runner.commands, 1, "ffmpeg is not run when the duration is unknown")
	}
}

func TestRenderReportsFFmpegFailures(t *testing.T) {
	runner := &fakeRunner{duration: "5", ffmpegErr: errors.New("ffmpeg: exit status 1: Unknown encoder")}
	dir := t.TempDir()

	_, err := renderer.NewFFmpegRendererWithRunner(rendererConfig(), runner.run).Render(context.Background(), generatorapi.RenderRequest{
		AudioPath: filepath.Join(dir, "a.mp3"), Narration: "Hello.", OutputPath: filepath.Join(dir, "v.mp4"),
	})

	require.ErrorContains(t, err, "Unknown encoder")
}

func TestLoadRendererConfig(t *testing.T) {
	defaults, err := renderer.LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, "ffmpeg", defaults.FfmpegPath)
	assert.Equal(t, "DejaVu Sans", defaults.CaptionFont)
	assert.Empty(t, defaults.BackgroundVideo)

	t.Setenv("GENERATOR_FFMPEG_PATH", "/usr/local/bin/ffmpeg")
	t.Setenv("GENERATOR_BACKGROUND_VIDEO", "/assets/bg.mp4")
	t.Setenv("GENERATOR_WORDS_PER_CAPTION", "8")
	cfg, err := renderer.LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, "/usr/local/bin/ffmpeg", cfg.FfmpegPath)
	assert.Equal(t, "/assets/bg.mp4", cfg.BackgroundVideo)
	assert.Equal(t, 8, cfg.WordsPerCaption)
}

// TestRenderWithRealFFmpeg renders an actual video when ffmpeg is installed. Nothing leaves
// the machine: the "narration" is a tone ffmpeg generates itself.
func TestRenderWithRealFFmpeg(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping the real ffmpeg render in short mode")
	}
	for _, tool := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not installed", tool)
		}
	}

	dir := t.TempDir()
	audio := filepath.Join(dir, "narration.mp3")
	tone := exec.Command("ffmpeg", "-y", "-loglevel", "error", "-f", "lavfi", "-i", "sine=frequency=440:duration=2", audio)
	out, err := tone.CombinedOutput()
	require.NoError(t, err, string(out))

	video := filepath.Join(dir, "video.mp4")
	duration, err := renderer.NewFFmpegRenderer(rendererConfig()).Render(context.Background(), generatorapi.RenderRequest{
		AudioPath:  audio,
		Narration:  "This is a real render. It has two captions: one, and then {another}.",
		OutputPath: video,
	})

	require.NoError(t, err)
	assert.InDelta(t, 2.0, duration.Seconds(), 0.3)

	probe, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "stream=codec_type,width,height", "-of", "csv=p=0", video).Output()
	require.NoError(t, err)
	assert.Contains(t, string(probe), "video,1080,1920", "the video is vertical 1080x1920")
	assert.Contains(t, string(probe), "audio", "the video has the narration")
}

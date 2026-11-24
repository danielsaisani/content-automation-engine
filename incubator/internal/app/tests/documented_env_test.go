package tests

import (
	"content-automation-engine/internal/content"
	"content-automation-engine/internal/content/scraper"
	"content-automation-engine/internal/generator"
	"content-automation-engine/internal/generator/narrator"
	"content-automation-engine/internal/generator/renderer"
	"content-automation-engine/internal/mongodb"
	"content-automation-engine/internal/poster"
	"content-automation-engine/internal/poster/tiktok"
	"content-automation-engine/internal/scheduler"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEveryDocumentedVariableIsHonoured sets each variable listed in the README to a
// non-default value and checks it reaches the configuration. envconfig derives variable
// names from field names, so a renamed field would otherwise silently change a variable.
func TestEveryDocumentedVariableIsHonoured(t *testing.T) {
	for name, value := range map[string]string{
		"MONGO_USERNAME": "user", "MONGO_PASSWORD": "pass", "MONGO_HOST": "host:1", "MONGO_APP_NAME": "app", "MONGO_DATABASE": "staging",
		"SCRAPER_APP_ID": "id", "SCRAPER_APP_SECRET": "secret", "SCRAPER_USERNAME": "user", "SCRAPER_PASSWORD": "pass",
		"SCRAPER_SUBREDDITS": "a,b", "SCRAPER_USER_AGENT": "agent/1", "SCRAPER_POLL_INTERVAL": "9s",
		"SCHEDULER_INTERVAL": "6h", "SCHEDULER_RUN_ON_START": "true",
		"CONTENT_CLAIM_TTL": "1h", "CONTENT_MAX_ATTEMPTS": "7", "CONTENT_MIN_BODY_CHARS": "11", "CONTENT_MAX_BODY_CHARS": "22",
		"GENERATOR_KOKORO_URL": "http://k:1", "GENERATOR_KOKORO_VOICE": "voice", "GENERATOR_KOKORO_MODEL": "model", "GENERATOR_KOKORO_TIMEOUT": "3m",
		"GENERATOR_OUTPUT_DIR": "/out", "GENERATOR_MAX_NARRATION_CHARS": "99",
		"GENERATOR_BACKGROUND_VIDEO": "/bg.mp4", "GENERATOR_BACKGROUND_COLOR": "0xffffff",
		"GENERATOR_CAPTION_FONT": "Arial", "GENERATOR_CAPTION_FONT_SIZE": "50", "GENERATOR_WORDS_PER_CAPTION": "9",
		"GENERATOR_FFMPEG_PATH": "/bin/ffmpeg", "GENERATOR_FFPROBE_PATH": "/bin/ffprobe",
		"POSTER_PLATFORM": "tiktok", "POSTER_HASHTAGS": "#x", "POSTER_MAX_CAPTION_CHARS": "77",
		"POSTER_TIKTOK_SESSION_ID": "sid", "POSTER_TIKTOK_DATACENTER_ID": "dc", "POSTER_TIKTOK_VISIBILITY": "2",
	} {
		t.Setenv(name, value)
	}

	mongoCfg, err := mongodb.LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, mongodb.Config{Username: "user", Password: "pass", Host: "host:1", AppName: "app", Database: "staging"}, *mongoCfg)

	scraperCfg, err := scraper.LoadRedditScraperConfig()
	require.NoError(t, err)
	assert.Equal(t, scraper.RedditScraperConfig{
		AppID: "id", AppSecret: "secret", Username: "user", Password: "pass",
		Subreddits: []string{"a", "b"}, UserAgent: "agent/1", PollInterval: 9 * time.Second,
	}, *scraperCfg)

	schedulerCfg, err := scheduler.LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, scheduler.Config{Interval: 6 * time.Hour, RunOnStart: true}, *schedulerCfg)

	contentCfg, err := content.LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, content.Config{ClaimTTL: time.Hour, MaxAttempts: 7, MinBodyChars: 11, MaxBodyChars: 22}, *contentCfg)

	kokoroCfg, err := narrator.LoadKokoroConfig()
	require.NoError(t, err)
	assert.Equal(t, narrator.KokoroConfig{URL: "http://k:1", Voice: "voice", Model: "model", Timeout: 3 * time.Minute}, *kokoroCfg)

	generatorCfg, err := generator.LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, generator.Config{OutputDir: "/out", MaxNarrationChars: 99}, *generatorCfg)

	rendererCfg, err := renderer.LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, renderer.Config{
		FfmpegPath: "/bin/ffmpeg", FfprobePath: "/bin/ffprobe", BackgroundVideo: "/bg.mp4", BackgroundColor: "0xffffff",
		CaptionFont: "Arial", CaptionFontSize: 50, WordsPerCaption: 9,
	}, *rendererCfg)

	posterCfg, err := poster.LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, poster.Config{Platform: "tiktok", Hashtags: "#x", MaxCaptionChars: 77}, *posterCfg)

	tiktokCfg, err := tiktok.LoadPlatformConfig()
	require.NoError(t, err)
	assert.Equal(t, tiktok.PlatformConfig{SessionID: "sid", DatacenterID: "dc", Visibility: 2, AllowComment: true}, *tiktokCfg)
}

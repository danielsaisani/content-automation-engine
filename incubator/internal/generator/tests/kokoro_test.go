package tests

import (
	"content-automation-engine/internal/generator/narrator"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func kokoroConfig(url string) *narrator.KokoroConfig {
	return &narrator.KokoroConfig{URL: url, Model: "kokoro", Voice: "af_heart", Timeout: 5 * time.Second}
}

func TestKokoroNarratorSavesTheAudioItReceives(t *testing.T) {
	var received map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/audio/speech", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		require.NoError(t, json.NewDecoder(r.Body).Decode(&received))
		_, _ = w.Write([]byte("mp3-bytes"))
	}))
	defer server.Close()
	output := filepath.Join(t.TempDir(), "narration.mp3")

	// A trailing slash on the configured URL must not produce a double slash.
	err := narrator.NewKokoroNarrator(kokoroConfig(server.URL+"/")).Narrate(context.Background(), "Hello there.", output)

	require.NoError(t, err)
	assert.Equal(t, map[string]string{"model": "kokoro", "input": "Hello there.", "voice": "af_heart", "response_format": "mp3"}, received)
	saved, err := os.ReadFile(output)
	require.NoError(t, err)
	assert.Equal(t, "mp3-bytes", string(saved))
}

func TestKokoroNarratorReportsServerErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "voice not found", http.StatusBadRequest)
	}))
	defer server.Close()
	output := filepath.Join(t.TempDir(), "narration.mp3")

	err := narrator.NewKokoroNarrator(kokoroConfig(server.URL)).Narrate(context.Background(), "Hello.", output)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "400")
	assert.Contains(t, err.Error(), "voice not found")
	assert.NoFileExists(t, output)
}

func TestKokoroNarratorRejectsAnEmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	err := narrator.NewKokoroNarrator(kokoroConfig(server.URL)).Narrate(context.Background(), "Hello.", filepath.Join(t.TempDir(), "a.mp3"))

	require.Error(t, err)
}

func TestKokoroNarratorRejectsEmptyText(t *testing.T) {
	err := narrator.NewKokoroNarrator(kokoroConfig("http://unused.invalid")).Narrate(context.Background(), "  ", filepath.Join(t.TempDir(), "a.mp3"))

	require.Error(t, err)
}

func TestKokoroNarratorReportsAnUnreachableServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close()

	err := narrator.NewKokoroNarrator(kokoroConfig(server.URL)).Narrate(context.Background(), "Hello.", filepath.Join(t.TempDir(), "a.mp3"))

	require.Error(t, err)
}

func TestLoadKokoroConfig(t *testing.T) {
	defaults, err := narrator.LoadKokoroConfig()
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:8880", defaults.URL)
	assert.Equal(t, "af_heart", defaults.Voice)

	t.Setenv("GENERATOR_KOKORO_URL", "http://kokoro:8880")
	t.Setenv("GENERATOR_KOKORO_VOICE", "am_adam")
	cfg, err := narrator.LoadKokoroConfig()
	require.NoError(t, err)
	assert.Equal(t, "http://kokoro:8880", cfg.URL)
	assert.Equal(t, "am_adam", cfg.Voice)
}

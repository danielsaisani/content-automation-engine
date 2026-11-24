package tests

import (
	"bytes"
	"content-automation-engine/internal/service"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONLoggerWritesStructuredLines(t *testing.T) {
	var out bytes.Buffer
	logger, err := service.NewLogger(&service.LogConfig{Format: "json", Level: "info"}, &out)
	require.NoError(t, err)

	logger.Info("Saved story", "storyID", "abc")

	var line map[string]any
	require.NoError(t, json.Unmarshal(out.Bytes(), &line))
	assert.Equal(t, "Saved story", line["msg"])
	assert.Equal(t, "abc", line["storyID"])
	assert.Equal(t, "INFO", line["level"])
}

func TestLoggerHonoursItsLevel(t *testing.T) {
	var out bytes.Buffer
	logger, err := service.NewLogger(&service.LogConfig{Format: "text", Level: "warn"}, &out)
	require.NoError(t, err)

	logger.Info("too quiet to show")
	assert.Empty(t, out.String())

	logger.Error("loud enough")
	assert.Contains(t, out.String(), "loud enough")
}

func TestLoggerRejectsBadSettings(t *testing.T) {
	var out bytes.Buffer

	_, err := service.NewLogger(&service.LogConfig{Format: "xml", Level: "info"}, &out)
	require.Error(t, err)

	_, err = service.NewLogger(&service.LogConfig{Format: "text", Level: "chatty"}, &out)
	require.Error(t, err)
}

func TestLoadLogConfig(t *testing.T) {
	defaults, err := service.LoadLogConfig()
	require.NoError(t, err)
	assert.Equal(t, "text", defaults.Format)
	assert.Equal(t, "info", defaults.Level)

	t.Setenv("LOG_FORMAT", "json")
	t.Setenv("LOG_LEVEL", "debug")
	cfg, err := service.LoadLogConfig()
	require.NoError(t, err)
	assert.Equal(t, "json", cfg.Format)
	assert.Equal(t, "debug", cfg.Level)
}

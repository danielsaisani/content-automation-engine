// Package narrator holds the text-to-speech implementations the generator can use.
package narrator

import (
	"bytes"
	"content-automation-engine/internal/generator/api"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/vrischmann/envconfig"
)

// KokoroConfig is read from environment variables prefixed with GENERATOR_KOKORO_.
type KokoroConfig struct {
	// URL is the base URL of a Kokoro-FastAPI server (or any server with an
	// OpenAI-compatible /v1/audio/speech endpoint).
	URL   string `envconfig:"default=http://localhost:8880"`
	Model string `envconfig:"default=kokoro"`
	Voice string `envconfig:"default=af_heart"`
	// Timeout bounds one narration request. CPU synthesis of a long story takes a while.
	Timeout time.Duration `envconfig:"default=10m"`
}

// LoadKokoroConfig reads the Kokoro narrator configuration from the environment.
func LoadKokoroConfig() (*KokoroConfig, error) {
	cfg := &KokoroConfig{}
	if err := envconfig.InitWithPrefix(cfg, "GENERATOR_KOKORO"); err != nil {
		return nil, fmt.Errorf("loading kokoro config: %w", err)
	}
	return cfg, nil
}

var _ api.Narrator = (*KokoroNarrator)(nil)

// KokoroNarrator narrates with the free, self-hosted Kokoro text-to-speech model.
type KokoroNarrator struct {
	cfg    *KokoroConfig
	client *http.Client
}

func NewKokoroNarrator(cfg *KokoroConfig) *KokoroNarrator {
	return &KokoroNarrator{cfg: cfg, client: &http.Client{Timeout: cfg.Timeout}}
}

type speechRequest struct {
	Model          string `json:"model"`
	Input          string `json:"input"`
	Voice          string `json:"voice"`
	ResponseFormat string `json:"response_format"`
}

func (n *KokoroNarrator) Narrate(ctx context.Context, text string, outputPath string) error {
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("narrating: text is empty")
	}

	body, err := json.Marshal(speechRequest{Model: n.cfg.Model, Input: text, Voice: n.cfg.Voice, ResponseFormat: "mp3"})
	if err != nil {
		return fmt.Errorf("narrating: %w", err)
	}

	endpoint := strings.TrimRight(n.cfg.URL, "/") + "/v1/audio/speech"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("narrating: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("narrating: calling %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("narrating: %s returned %s: %s", endpoint, resp.Status, strings.TrimSpace(string(detail)))
	}

	out, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("narrating: %w", err)
	}
	written, copyErr := io.Copy(out, resp.Body)
	closeErr := out.Close()
	if copyErr != nil {
		return fmt.Errorf("narrating: saving audio: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("narrating: saving audio: %w", closeErr)
	}
	if written == 0 {
		return fmt.Errorf("narrating: %s returned no audio", endpoint)
	}
	return nil
}

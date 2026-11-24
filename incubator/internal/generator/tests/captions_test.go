package tests

import (
	"content-automation-engine/internal/generator/captions"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanStripsLinksMarkdownAndEntities(t *testing.T) {
	raw := "**AITA** for [this](https://example.com/x)?\n\n> quoted &amp; `code` see https://reddit.com/r/x  now"

	assert.Equal(t, "AITA for this? quoted & code see now", captions.Clean(raw))
}

func TestTruncateCutsAtTheEndOfASentence(t *testing.T) {
	text := "First sentence. Second sentence is here. Third one."

	assert.Equal(t, "First sentence. Second sentence is here.", captions.Truncate(text, 45))
	assert.Equal(t, text, captions.Truncate(text, 500), "short text is left alone")
	assert.Equal(t, text, captions.Truncate(text, 0), "zero means no limit")
}

func TestTruncateFallsBackToAWholeWord(t *testing.T) {
	assert.Equal(t, "one two", captions.Truncate("one two three four", 10))
}

func TestSplitLimitsWordsAndBreaksOnSentences(t *testing.T) {
	got := captions.Split("I went home. Then I slept for a very long time indeed!", 4)

	assert.Equal(t, []string{"I went home.", "Then I slept for", "a very long time", "indeed!"}, got)
}

func TestSplitOfEmptyTextIsEmpty(t *testing.T) {
	assert.Empty(t, captions.Split("   ", 5))
}

func TestASSSpreadsCaptionsOverTheWholeDuration(t *testing.T) {
	ass := captions.ASS([]string{"aaaa", "bbbbbbbb", "cccc"}, 16*time.Second, captions.DefaultStyle)

	var dialogue []string
	for _, line := range strings.Split(ass, "\n") {
		if strings.HasPrefix(line, "Dialogue:") {
			dialogue = append(dialogue, line)
		}
	}
	require.Len(t, dialogue, 3)
	// 16 characters over 16 seconds: each caption is on screen for as long as its share of the text.
	assert.Contains(t, dialogue[0], "0:00:00.00,0:00:04.00")
	assert.Contains(t, dialogue[1], "0:00:04.00,0:00:12.00")
	assert.Contains(t, dialogue[2], "0:00:12.00,0:00:16.00")
	assert.Contains(t, ass, "PlayResX: 1080")
	assert.Contains(t, ass, "PlayResY: 1920")
}

func TestASSEscapesFormattingCharacters(t *testing.T) {
	ass := captions.ASS([]string{`{\b1}bold{\b0}`}, time.Second, captions.DefaultStyle)

	assert.Contains(t, ass, ",,(b1)bold(b0)\n")
}

func TestASSFormatsLongTimestamps(t *testing.T) {
	ass := captions.ASS([]string{"a", "b"}, 2*time.Hour+2*time.Minute+3*time.Second, captions.DefaultStyle)

	assert.Contains(t, ass, "1:01:01.50,2:02:03.00")
}

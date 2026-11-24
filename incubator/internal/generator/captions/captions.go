// Package captions prepares story text for narration and on-screen captions.
package captions

import (
	"fmt"
	"html"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	markdownLink = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	bareURL      = regexp.MustCompile(`https?://\S+`)
	markup       = regexp.MustCompile("[*_~`#>|\\\\]+")
	whitespace   = regexp.MustCompile(`\s+`)
)

// Clean strips links, markdown and HTML entities from scraped text so it reads well aloud.
func Clean(text string) string {
	text = html.UnescapeString(text)
	text = markdownLink.ReplaceAllString(text, "$1")
	text = bareURL.ReplaceAllString(text, "")
	text = markup.ReplaceAllString(text, " ")
	text = whitespace.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

// Truncate shortens text to at most maxChars characters, cutting at the end of a sentence
// when there is one, otherwise at the last whole word.
func Truncate(text string, maxChars int) string {
	if maxChars <= 0 || utf8.RuneCountInString(text) <= maxChars {
		return text
	}

	cut := string([]rune(text)[:maxChars])
	if end := strings.LastIndexAny(cut, ".!?"); end > 0 {
		return strings.TrimSpace(cut[:end+1])
	}
	if end := strings.LastIndex(cut, " "); end > 0 {
		return strings.TrimSpace(cut[:end])
	}
	return cut
}

// Split breaks text into captions of at most maxWords words, ending a caption early at the
// end of a sentence so captions follow the rhythm of the narration.
func Split(text string, maxWords int) []string {
	if maxWords <= 0 {
		maxWords = 6
	}

	var captions []string
	var current []string
	flush := func() {
		if len(current) > 0 {
			captions = append(captions, strings.Join(current, " "))
			current = nil
		}
	}

	for _, word := range strings.Fields(text) {
		current = append(current, word)
		endsSentence := strings.HasSuffix(word, ".") || strings.HasSuffix(word, "!") || strings.HasSuffix(word, "?")
		if len(current) >= maxWords || endsSentence {
			flush()
		}
	}
	flush()

	return captions
}

// Style controls how captions look.
type Style struct {
	Font     string
	FontSize int
	Width    int
	Height   int
}

// DefaultStyle is large white text in the middle of a vertical 1080x1920 video.
var DefaultStyle = Style{Font: "DejaVu Sans", FontSize: 84, Width: 1080, Height: 1920}

// ASS builds an Advanced SubStation Alpha subtitle file showing each caption in turn.
//
// There are no word timestamps from the narrator, so each caption is given a share of the
// total duration in proportion to its length. That keeps captions close to the voice.
func ASS(captions []string, total time.Duration, style Style) string {
	var b strings.Builder

	fmt.Fprintf(&b, "[Script Info]\nScriptType: v4.00+\nPlayResX: %d\nPlayResY: %d\nWrapStyle: 0\n\n", style.Width, style.Height)
	b.WriteString("[V4+ Styles]\n")
	b.WriteString("Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n")
	fmt.Fprintf(&b, "Style: Default,%s,%d,&H00FFFFFF,&H000000FF,&H00000000,&H64000000,-1,0,0,0,100,100,0,0,1,6,2,5,90,90,0,1\n\n", style.Font, style.FontSize)
	b.WriteString("[Events]\n")
	b.WriteString("Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n")

	totalChars := 0
	for _, caption := range captions {
		totalChars += utf8.RuneCountInString(caption)
	}
	if totalChars == 0 {
		return b.String()
	}

	shown := 0
	for _, caption := range captions {
		start := time.Duration(float64(total) * float64(shown) / float64(totalChars))
		shown += utf8.RuneCountInString(caption)
		end := time.Duration(float64(total) * float64(shown) / float64(totalChars))
		fmt.Fprintf(&b, "Dialogue: 0,%s,%s,Default,,0,0,0,,%s\n", assTime(start), assTime(end), assEscape(caption))
	}

	return b.String()
}

// assTime formats a duration as H:MM:SS.cc, the timestamp format ASS uses.
func assTime(d time.Duration) string {
	centis := d.Milliseconds() / 10
	return fmt.Sprintf("%d:%02d:%02d.%02d", centis/360000, (centis/6000)%60, (centis/100)%60, centis%100)
}

// assEscape removes the characters ASS would treat as formatting codes.
func assEscape(text string) string {
	return strings.NewReplacer("{", "(", "}", ")", "\\", "", "\n", " ").Replace(text)
}

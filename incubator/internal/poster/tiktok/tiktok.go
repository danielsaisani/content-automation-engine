package tiktok

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
)

type TiktokConfig struct {
	LoginURL                       string
	ProjectURL                     string
	Cookies                        TiktokCookies
	VideosDir                      string
	PostProcessingVideoPath        string
	ImagemagickFont                string
	ImagemagickFontSize            int
	ImagemagickTextForegroundColor string
	ImagemagickTextBackgroundColor string
	ImagemagickBinary              string
}

type TiktokClient struct {
	Config *TiktokConfig
}

type TiktokCookies map[string]string

// GetSessionID returns the Tiktok session ID, if not found, returns empty string
func (tc *TiktokCookies) GetSessionID() string {
	return (*tc)["sessionid"]
}

// GetDCID returns the Tiktok datacenter ID, if not found, returns empty string
func (tc *TiktokCookies) GetDCID() string {
	return (*tc)["tt-target-idc"]
}

func NewTiktokClient() *TiktokClient {

	fontSize, err := strconv.Atoi(os.Getenv("IMAGEMAGICK_FONT_SIZE"))
	if err != nil {
		log.Fatalf("Failed to convert IMAGEMAGICK_FONT_SIZE to int: %v", err)
	}

	config := &TiktokConfig{
		LoginURL:                       os.Getenv("TIKTOK_LOGIN_URL"),
		ProjectURL:                     os.Getenv("TIKTOK_PROJECT_URL"),
		Cookies:                        TiktokCookies{},
		VideosDir:                      os.Getenv("VIDEOS_DIR"),
		PostProcessingVideoPath:        os.Getenv("POST_PROCESSING_VIDEO_PATH"),
		ImagemagickFont:                os.Getenv("IMAGEMAGICK_FONT"),
		ImagemagickFontSize:            fontSize,
		ImagemagickTextForegroundColor: os.Getenv("IMAGEMAGICK_TEXT_FOREGROUND_COLOR"),
		ImagemagickTextBackgroundColor: os.Getenv("IMAGEMAGICK_TEXT_BACKGROUND_COLOR"),
		ImagemagickBinary:              os.Getenv("IMAGEMAGICK_BINARY"),
	}
	return &TiktokClient{Config: config}
}

// Login logs in to Tiktok and saves the cookies to the TiktokClient's config
func (c *TiktokClient) Login(ctx context.Context) error {
	return nil
}

func (c *TiktokClient) UploadVideo(ctx context.Context, videoPath string) error {
	_, err := GetUserAgent()
	if err != nil {
		return fmt.Errorf("failed to get user agent: %w", err)
	}

	sessionID := c.Config.Cookies.GetSessionID()
	dcID := c.Config.Cookies.GetDCID()

	if sessionID == "" {
		return fmt.Errorf("session ID not found")
	}

	if dcID == "" {
		return fmt.Errorf("datacenter ID not found")
	}

	return nil
}

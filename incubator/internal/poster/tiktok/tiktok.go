package tiktok

import (
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

	fontSize, _ := strconv.Atoi(os.Getenv("IMAGEMAGICK_FONT_SIZE"))

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

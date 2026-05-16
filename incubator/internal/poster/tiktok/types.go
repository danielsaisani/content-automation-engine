package tiktok

import "time"

// TiktokPostInput represents the input data for posting a video to TikTok.
type TiktokPostInput struct {
	VideoPath         string
	Title             string
	ScheduleTime      *time.Time
	AllowComment      bool
	AllowDuet         bool
	AllowStitch       bool
	VisibilityType    int // 0: Public, 1: Private, 2: Friends
	BrandOrganicType  int
	BrandedContentType int
	AILabel           int
	Proxy             string
}

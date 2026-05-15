package tiktok

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type ProjectCreateResponse struct {
	Project struct {
		ProjectID string `json:"project_id"`
	} `json:"project"`
}

type PublishResponse struct {
	StatusCode int    `json:"status_code"`
	StatusMsg  string `json:"status_msg"`
}

func (c *TiktokClient) UploadVideo(ctx context.Context, input *TiktokPostInput) error {
	c.logger.Info("Starting TikTok video upload", "title", input.Title)

	userAgent, _ := GetUserAgent()

	// Create HTTP client with proxy if needed
	httpClient := &http.Client{
		Timeout: 30 * time.Minute, // Large timeout for video uploads
	}
	if input.Proxy != "" {
		proxyURL, err := url.Parse(input.Proxy)
		if err == nil {
			httpClient.Transport = &http.Transport{
				Proxy: http.ProxyURL(proxyURL),
			}
			c.logger.Info("Using proxy for upload", "proxy", input.Proxy)
		}
	}

	// 1. Create Project
	creationID := generateRandomString(21)
	projectURL := fmt.Sprintf("https://www.tiktok.com/api/v1/web/project/create/?creation_id=%s&type=1&aid=1988", creationID)
	req, _ := http.NewRequestWithContext(ctx, "POST", projectURL, nil)
	c.setHeaders(req)
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to create project: %w", err)
	}
	defer resp.Body.Close()

	var projectResp ProjectCreateResponse
	if err := json.NewDecoder(resp.Body).Decode(&projectResp); err != nil {
		return fmt.Errorf("failed to decode project response: %w", err)
	}
	c.logger.Debug("Project created", "projectID", projectResp.Project.ProjectID)

	// 2. Upload Video to TikTok storage
	videoID, sessionKey, uploadID, crcs, uploadHost, storeURI, videoAuth, awsSigner, awsCreds, err := c.UploadToTiktok(ctx, input.VideoPath, httpClient)
	if err != nil {
		return fmt.Errorf("failed to upload to tiktok storage: %w", err)
	}

	// 3. Finish Upload Phase
	finishURL := fmt.Sprintf("https://%s/%s?uploadID=%s&phase=finish&uploadmode=part", uploadHost, storeURI, uploadID)
	var crcStrings []string
	for i, crc := range crcs {
		crcStrings = append(crcStrings, fmt.Sprintf("%d:%08x", i+1, crc))
	}
	finishReq, _ := http.NewRequestWithContext(ctx, "POST", finishURL, bytes.NewBufferString(strings.Join(crcStrings, ",")))
	finishReq.Header.Set("Authorization", videoAuth)
	finishReq.Header.Set("Content-Type", "text/plain;charset=UTF-8")
	resp, err = httpClient.Do(finishReq)
	if err != nil {
		return fmt.Errorf("failed to finish upload: %w", err)
	}
	resp.Body.Close()
	c.logger.Debug("Upload phase finished")

	// 4. Commit Upload Inner
	commitURL := "https://www.tiktok.com/top/v1?Action=CommitUploadInner&Version=2020-11-19&SpaceName=tiktok"
	commitData := fmt.Sprintf(`{"SessionKey":"%s","Functions":[{"name":"GetMeta"}]}`, sessionKey)
	commitReq, _ := http.NewRequestWithContext(ctx, "POST", commitURL, bytes.NewBufferString(commitData))
	c.setHeaders(commitReq)

	creds, _ := awsCreds.Retrieve(ctx)
	payloadHash := fmt.Sprintf("%x", sha256Sum([]byte(commitData)))
	err = awsSigner.SignHTTP(ctx, creds, commitReq, payloadHash, "vod", "ap-singapore-1", time.Now())
	if err != nil {
		return fmt.Errorf("failed to sign commit request: %w", err)
	}

	resp, err = httpClient.Do(commitReq)
	if err != nil {
		return fmt.Errorf("failed to commit upload: %w", err)
	}
	resp.Body.Close()
	c.logger.Debug("Upload committed")

	// 5. Publish Video
	postData := map[string]interface{}{
		"post_common_info": map[string]interface{}{
			"creation_id":           creationID,
			"enter_post_page_from": 1,
			"post_type":             3,
		},
		"feature_common_info_list": []interface{}{
			map[string]interface{}{
				"geofencing_regions": []string{},
				"playlist_name":      "",
				"playlist_id":        "",
				"tcm_params":         "{\"commerce_toggle_info\":{}}",
				"sound_exemption":    0,
				"anchors":            []string{},
				"vedit_common_info": map[string]interface{}{
					"draft":    "",
					"video_id": videoID,
				},
				"privacy_setting_info": map[string]interface{}{
					"visibility_type": input.VisibilityType,
					"allow_duet":      boolToInt(input.AllowDuet),
					"allow_stitch":    boolToInt(input.AllowStitch),
					"allow_comment":   boolToInt(input.AllowComment),
				},
			},
		},
		"single_post_req_list": []interface{}{
			map[string]interface{}{
				"batch_index": 0,
				"video_id":    videoID,
				"is_long_video": 0,
				"single_post_feature_info": map[string]interface{}{
					"text":        input.Title,
					"markup_text": input.Title,
					"music_info":  map[string]interface{}{},
					"poster_delay": 0,
				},
			},
		},
	}

	if input.ScheduleTime != nil {
		postData["feature_common_info_list"].([]interface{})[0].(map[string]interface{})["schedule_time"] = input.ScheduleTime.Unix()
		c.logger.Info("Scheduling video", "time", input.ScheduleTime)
	}

	// Sign the publish request
	mstoken := "" // retrieved from cookies if needed
	sigURL := fmt.Sprintf("https://www.tiktok.com/api/v1/web/project/post/?app_name=tiktok_web&channel=tiktok_web&device_platform=web&aid=1988&msToken=%s", mstoken)
	sig, err := c.GenerateSignatures(ctx, sigURL, string(userAgent))
	if err != nil {
		return fmt.Errorf("failed to generate publish signatures: %w", err)
	}

	publishURL := "https://www.tiktok.com/tiktok/web/project/post/v1/"
	publishBody, _ := json.Marshal(postData)
	publishReq, _ := http.NewRequestWithContext(ctx, "POST", publishURL, bytes.NewBuffer(publishBody))
	c.setHeaders(publishReq)

	q := publishReq.URL.Query()
	q.Add("app_name", "tiktok_web")
	q.Add("channel", "tiktok_web")
	q.Add("device_platform", "web")
	q.Add("aid", "1988")
	q.Add("X-Bogus", sig.Bogus)
	q.Add("_signature", sig.Signature)
	publishReq.URL.RawQuery = q.Encode()
	publishReq.Header.Set("Content-Type", "application/json")

	resp, err = httpClient.Do(publishReq)
	if err != nil {
		return fmt.Errorf("failed to publish video: %w", err)
	}
	defer resp.Body.Close()

	var pubResp PublishResponse
	if err := json.NewDecoder(resp.Body).Decode(&pubResp); err != nil {
		return fmt.Errorf("failed to decode publish response: %w", err)
	}

	if pubResp.StatusCode != 0 {
		return fmt.Errorf("publish failed: %s (code %d)", pubResp.StatusMsg, pubResp.StatusCode)
	}

	c.logger.Info("TikTok video published successfully")
	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func sha256Sum(data []byte) []byte {
	h := sha256.New()
	h.Write(data)
	return h.Sum(nil)
}

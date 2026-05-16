package tiktok

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/aws/aws-sdk-go-v2/credentials"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
)

type UploadAuthResponse struct {
	VideoTokenV5 struct {
		AccessKeyID     string `json:"access_key_id"`
		SecretAccessKey string `json:"secret_acess_key"` // tiktok's typo: acess
		SessionToken    string `json:"session_token"`
	} `json:"video_token_v5"`
}

type ApplyUploadInnerResponse struct {
	Result struct {
		InnerUploadAddress struct {
			UploadNodes []struct {
				Vid        string `json:"Vid"`
				UploadHost string `json:"UploadHost"`
				SessionKey string `json:"SessionKey"`
				StoreInfos []struct {
					StoreUri string `json:"StoreUri"`
					Auth     string `json:"Auth"`
				} `json:"StoreInfos"`
			} `json:"UploadNodes"`
		} `json:"InnerUploadAddress"`
	} `json:"Result"`
}

func (c *TiktokClient) UploadToTiktok(ctx context.Context, videoFile string, httpClient *http.Client) (string, string, string, []uint32, string, string, string, *v4.Signer, credentials.StaticCredentialsProvider, error) {
	c.logger.Info("Starting upload to TikTok storage", "videoFile", videoFile)

	// 1. Get Upload Auth
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://www.tiktok.com/api/v1/video/upload/auth/?aid=1988", nil)
	c.setHeaders(req)
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", "", "", nil, "", "", "", nil, credentials.StaticCredentialsProvider{}, err
	}
	defer resp.Body.Close()

	var authResp UploadAuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return "", "", "", nil, "", "", "", nil, credentials.StaticCredentialsProvider{}, err
	}

	awsCreds := credentials.NewStaticCredentialsProvider(
		authResp.VideoTokenV5.AccessKeyID,
		authResp.VideoTokenV5.SecretAccessKey,
		authResp.VideoTokenV5.SessionToken,
	)
	awsSigner := v4.NewSigner()

	// 2. Stat Video File to get size
	videoPath := videoFile
	if !filepath.IsAbs(videoPath) {
		videoPath = filepath.Join(c.Config.VideosDir, videoFile)
	}
	fileInfo, err := os.Stat(videoPath)
	if err != nil {
		return "", "", "", nil, "", "", "", nil, credentials.StaticCredentialsProvider{}, err
	}
	fileSize := fileInfo.Size()

	// 3. Apply Upload Inner
	applyURL := fmt.Sprintf("https://www.tiktok.com/top/v1?Action=ApplyUploadInner&Version=2020-11-19&SpaceName=tiktok&FileType=video&IsInner=1&FileSize=%d&s=g158iqx8434", fileSize)
	applyReq, _ := http.NewRequestWithContext(ctx, "GET", applyURL, nil)
	c.setHeaders(applyReq)

	// Sign ApplyUploadInner request with AWS SigV4
	payloadHash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" // SHA256 of empty body
	creds, _ := awsCreds.Retrieve(ctx)
	err = awsSigner.SignHTTP(ctx, creds, applyReq, payloadHash, "vod", "ap-singapore-1", time.Now())
	if err != nil {
		return "", "", "", nil, "", "", "", nil, credentials.StaticCredentialsProvider{}, err
	}

	resp, err = httpClient.Do(applyReq)
	if err != nil {
		return "", "", "", nil, "", "", "", nil, credentials.StaticCredentialsProvider{}, err
	}
	defer resp.Body.Close()

	var applyResp ApplyUploadInnerResponse
	if err := json.NewDecoder(resp.Body).Decode(&applyResp); err != nil {
		return "", "", "", nil, "", "", "", nil, credentials.StaticCredentialsProvider{}, err
	}

	if len(applyResp.Result.InnerUploadAddress.UploadNodes) == 0 {
		return "", "", "", nil, "", "", "", nil, credentials.StaticCredentialsProvider{}, fmt.Errorf("no upload nodes found")
	}

	node := applyResp.Result.InnerUploadAddress.UploadNodes[0]
	videoID := node.Vid
	uploadHost := node.UploadHost
	sessionKey := node.SessionKey
	storeURI := node.StoreInfos[0].StoreUri
	videoAuth := node.StoreInfos[0].Auth
	uploadID := uuid.New().String()

	// 4. Upload Chunks (Streaming)
	f, err := os.Open(videoPath)
	if err != nil {
		return "", "", "", nil, "", "", "", nil, credentials.StaticCredentialsProvider{}, err
	}
	defer f.Close()

	chunkSize := 5242880
	var crcs []uint32
	buffer := make([]byte, chunkSize)

	for i := 0; ; i++ {
		n, err := f.Read(buffer)
		if n == 0 {
			break
		}
		if err != nil && err.Error() != "EOF" {
			return "", "", "", nil, "", "", "", nil, credentials.StaticCredentialsProvider{}, err
		}

		chunk := buffer[:n]
		crc := crc32.ChecksumIEEE(chunk)
		crcs = append(crcs, crc)

		partNumber := i + 1
		chunkURL := fmt.Sprintf("https://%s/%s?partNumber=%d&uploadID=%s&phase=transfer", uploadHost, storeURI, partNumber, uploadID)

		c.logger.Debug("Uploading chunk", "partNumber", partNumber, "size", n)

		chunkReq, _ := http.NewRequestWithContext(ctx, "POST", chunkURL, bytes.NewReader(chunk))
		chunkReq.Header.Set("Authorization", videoAuth)
		chunkReq.Header.Set("Content-Type", "application/octet-stream")
		chunkReq.Header.Set("Content-Disposition", `attachment; filename="undefined"`)
		chunkReq.Header.Set("Content-Crc32", fmt.Sprintf("%08x", crc))

		resp, err = httpClient.Do(chunkReq)
		if err != nil {
			return "", "", "", nil, "", "", "", nil, credentials.StaticCredentialsProvider{}, err
		}
		resp.Body.Close()

		if n < chunkSize {
			break
		}
	}

	c.logger.Info("Video chunks uploaded successfully", "videoID", videoID)
	return videoID, sessionKey, uploadID, crcs, uploadHost, storeURI, videoAuth, awsSigner, awsCreds, nil
}

func (c *TiktokClient) setHeaders(req *http.Request) {
	req.Header.Set("User-Agent", defaultUserAgent)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	if c.Config.Cookies.GetSessionID() != "" {
		req.AddCookie(&http.Cookie{Name: "sessionid", Value: c.Config.Cookies.GetSessionID()})
	}
	if c.Config.Cookies.GetDCID() != "" {
		req.AddCookie(&http.Cookie{Name: "tt-target-idc", Value: c.Config.Cookies.GetDCID()})
	}
}

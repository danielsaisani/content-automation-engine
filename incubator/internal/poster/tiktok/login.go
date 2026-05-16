package tiktok

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// Login logs in to Tiktok and saves the cookies to the TiktokClient's config
func (c *TiktokClient) Login(ctx context.Context, loginName string) (TiktokCookies, error) {
	// Check if cookies already exist
	cookiesPath := filepath.Join("tiktok_sessions", fmt.Sprintf("tiktok_session-%s.json", loginName))
	if _, err := os.Stat(cookiesPath); err == nil {
		data, err := os.ReadFile(cookiesPath)
		if err == nil {
			var cookies TiktokCookies
			if err := json.Unmarshal(data, &cookies); err == nil {
				c.Config.Cookies = cookies
				return cookies, nil
			}
		}
	}

	// 1. Prepare Chromedp
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Flag("headless", false),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(ctx, opts...)
	defer cancel()

	taskCtx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	c.logger.Info("Opening browser for TikTok login", "url", c.Config.LoginURL)

	var collectedCookies TiktokCookies = make(TiktokCookies)

	err := chromedp.Run(taskCtx,
		chromedp.Navigate(c.Config.LoginURL),
		chromedp.ActionFunc(func(ctx context.Context) error {
			for {
				cookies, err := network.GetCookies().Do(ctx)
				if err != nil {
					return err
				}

				hasSessionID := false
				hasDCID := false

				for _, cookie := range cookies {
					if cookie.Name == "sessionid" {
						collectedCookies["sessionid"] = cookie.Value
						hasSessionID = true
					}
					if cookie.Name == "tt-target-idc" {
						collectedCookies["tt-target-idc"] = cookie.Value
						hasDCID = true
					}
				}

				if hasSessionID && hasDCID {
					break
				}

				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(1 * time.Second):
				}
			}
			return nil
		}),
	)

	if err != nil {
		return nil, fmt.Errorf("login failed: %w", err)
	}

	// Save cookies
	os.MkdirAll("tiktok_sessions", 0755)
	data, _ := json.Marshal(collectedCookies)
	os.WriteFile(cookiesPath, data, 0644)

	c.Config.Cookies = collectedCookies
	c.logger.Info("Login successful, cookies saved", "loginName", loginName)
	return collectedCookies, nil
}

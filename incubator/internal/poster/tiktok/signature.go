package tiktok

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/chromedp/chromedp"
)

//go:embed js/*.js
var jsFiles embed.FS

type Signatures struct {
	Signature string `json:"signature"`
	VerifyFP  string `json:"verify_fp"`
	SignedURL string `json:"signed_url"`
	Bogus     string `json:"x-bogus"`
	TTParams  string `json:"x-tt-params"`
}

func (c *TiktokClient) GenerateSignatures(ctx context.Context, targetURL string, userAgent string) (*Signatures, error) {
	c.logger.Debug("Generating signatures", "url", targetURL)

	// 1. Prepare Chromedp
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.UserAgent(userAgent),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(ctx, opts...)
	defer cancel()

	taskCtx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	// 2. Load the scripts from embedded FS
	signerJS, err := jsFiles.ReadFile("js/signer.js")
	if err != nil {
		return nil, fmt.Errorf("failed to read signer.js: %w", err)
	}
	webmssdkJS, err := jsFiles.ReadFile("js/webmssdk.js")
	if err != nil {
		return nil, fmt.Errorf("failed to read webmssdk.js: %w", err)
	}
	xbogusJS, err := jsFiles.ReadFile("js/xbogus.js")
	if err != nil {
		return nil, fmt.Errorf("failed to read xbogus.js: %w", err)
	}

	var res struct {
		Signature string `json:"signature"`
		Bogus     string `json:"bogus"`
	}
	verifyFP := "verify_5b161567bda98b6a50c0414d99909d4b"

	err = chromedp.Run(taskCtx,
		chromedp.Navigate("https://www.tiktok.com/@rihanna?lang=en"),
		chromedp.Evaluate(string(webmssdkJS), nil),
		chromedp.Evaluate(string(xbogusJS), nil),
		chromedp.Evaluate(string(signerJS), nil),
		chromedp.EvaluateAsDevTools(fmt.Sprintf(`
			(function() {
				const targetURL = %s;
				const verifyFP = %s;
				const userAgent = %s;
				const url = targetURL + "&verifyFp=" + verifyFP;
				const token = window.byted_acrawler.sign({ url: url });
				const signed_url = url + "&_signature=" + token;
				const queryString = new URL(signed_url).searchParams.toString();
				const bogus = window.generateBogus(queryString, userAgent);
				return { signature: token, bogus: bogus };
			})()
		`, quoteJS(targetURL), quoteJS(verifyFP), quoteJS(userAgent)), &res),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to execute signature JS: %w", err)
	}

	signedURL := fmt.Sprintf("%s&verifyFp=%s&_signature=%s&X-Bogus=%s", targetURL, verifyFP, res.Signature, res.Bogus)

	u, err := url.Parse(signedURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse signed URL: %w", err)
	}

	ttParams, err := c.generateTTParams(u.Query().Encode())
	if err != nil {
		return nil, fmt.Errorf("failed to generate x-tt-params: %w", err)
	}

	c.logger.Debug("Signatures generated successfully")
	return &Signatures{
		Signature: res.Signature,
		VerifyFP:  verifyFP,
		SignedURL: signedURL,
		Bogus:     res.Bogus,
		TTParams:  ttParams,
	}, nil
}

func quoteJS(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func (c *TiktokClient) generateTTParams(queryString string) (string, error) {
	queryString += "&is_encryption=1"
	password := "webapp1.0+202106"

	block, err := aes.NewCipher([]byte(password))
	if err != nil {
		return "", err
	}

	padding := aes.BlockSize - (len(queryString) % aes.BlockSize)
	padtext := append([]byte(queryString), make([]byte, padding)...)
	for i := len(queryString); i < len(padtext); i++ {
		padtext[i] = byte(padding)
	}

	ciphertext := make([]byte, len(padtext))
	mode := cipher.NewCBCEncrypter(block, []byte(password))
	mode.CryptBlocks(ciphertext, padtext)

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func generateRandomString(n int) string {
	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}

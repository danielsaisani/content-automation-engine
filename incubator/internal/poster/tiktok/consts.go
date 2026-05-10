package tiktok

type UserAgent string

// GetUserAgent returns the default user agent
func GetUserAgent() (UserAgent, error) {
	return UserAgent(defaultUserAgent), nil
}

const defaultUserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/68.0.3440.106 Safari/537.36"

package internal

import (
	"time"

	"github.com/marksalpeter/token/v2"
)

var tokenCache *TTLCache

func init() {
	// #244: How to make discoverability via user agents work again?
	tokenCache = NewTTLCache(1 * time.Hour)
}

func GenerateToken(feedurl string) string {
	t := token.New().Encode()
	for {
		if tokenCache.GetString(t) == "" {
			tokenCache.SetString(t, feedurl)
			return t
		}
		t = token.New().Encode()
	}
}

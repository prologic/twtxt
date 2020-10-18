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

func GenerateToken() string {
	t := token.New()
	ts := t.Encode()

	for {
		if tokenCache.Get(ts) == 0 {
			tokenCache.Set(ts, 1)
			return ts
		}
		t = token.New()
		ts = t.Encode()
	}
}

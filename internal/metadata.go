package internal

import "github.com/prologic/twtxt/types"

func TwtMetaDataFactory(conf *Config, cache *Cache) func(twt types.Twt) string {
	return func(twt types.Twt) string {
		return ""
	}
}

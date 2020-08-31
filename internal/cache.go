package internal

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/prologic/twtxt/types"
)

// Cached ...
type Cached struct {
	Twts         types.Twts
	Lastmodified string
}

// Cache key: url
type Cache map[string]Cached

// Store ...
func (cache Cache) Store(path string) error {
	b := new(bytes.Buffer)
	enc := gob.NewEncoder(b)
	err := enc.Encode(cache)
	if err != nil {
		log.WithError(err).Error("error encoding cache")
		return err
	}

	f, err := os.OpenFile(filepath.Join(path, "cache"), os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.WithError(err).Error("error opening cache file for writing")
		return err
	}

	defer f.Close()

	if _, err = f.Write(b.Bytes()); err != nil {
		log.WithError(err).Error("error writing cache file")
		return err
	}
	return nil
}

// CacheLastModified ...
func CacheLastModified(path string) (time.Time, error) {
	stat, err := os.Stat(filepath.Join(path, "cache"))
	if err != nil {
		if !os.IsNotExist(err) {
			return time.Time{}, err
		}
		return time.Unix(0, 0), nil
	}
	return stat.ModTime(), nil
}

// LoadCache ...
func LoadCache(path string) (Cache, error) {
	cache := make(Cache)

	f, err := os.Open(filepath.Join(path, "cache"))
	if err != nil {
		if !os.IsNotExist(err) {
			log.WithError(err).Error("error loading cache, cache not found")
			return nil, err
		}
		return cache, nil
	}
	defer f.Close()

	dec := gob.NewDecoder(f)
	err = dec.Decode(&cache)
	if err != nil {
		log.WithError(err).Error("error decoding cache")
		return nil, err
	}
	return cache, nil
}

const maxfetchers = 50

// FetchTwts ...
func (cache Cache) FetchTwts(conf *Config, archive Archiver, feeds types.Feeds) {
	var mu sync.RWMutex

	stime := time.Now()
	defer func() {
		metrics.Gauge(
			"cache",
			"last_processed_seconds",
		).Set(
			float64(time.Now().Sub(stime) / 1e9),
		)
	}()

	// buffered to let goroutines write without blocking before the main thread
	// begins reading
	twtsch := make(chan types.Twts, len(feeds))

	var wg sync.WaitGroup
	// max parallel http fetchers
	var fetchers = make(chan struct{}, maxfetchers)

	metrics.Gauge("cache", "sources").Set(float64(len(feeds)))

	for feed := range feeds {
		wg.Add(1)
		fetchers <- struct{}{}

		// anon func takes needed variables as arg, avoiding capture of iterator variables
		go func(feed types.Feed) {
			stime := time.Now()
			log.Infof("fetching feed %s", feed)

			defer func() {
				<-fetchers
				wg.Done()
				log.Infof("fetched feed %s (%s)", feed, time.Now().Sub(stime))
			}()

			headers := make(http.Header)

			mu.RLock()
			if cached, ok := cache[feed.URL]; ok {
				if cached.Lastmodified != "" {
					headers.Set("If-Modified-Since", cached.Lastmodified)
				}
			}
			mu.RUnlock()

			res, err := Request(conf, http.MethodGet, feed.URL, headers)
			if err != nil {
				log.WithError(err).Errorf("error fetching feed %s", feed)
				twtsch <- nil
				return
			}
			defer res.Body.Close()

			actualurl := res.Request.URL.String()
			if actualurl != feed.URL {
				log.WithError(err).Errorf("feed for %s changed from %s to %s", feed.Nick, feed.URL, actualurl)
				feed.URL = actualurl
			}

			if feed.URL == "" {
				log.WithField("feed", feed).Warn("empty url")
				twtsch <- nil
				return
			}

			var twts types.Twts

			switch res.StatusCode {
			case http.StatusOK: // 200
				limitedReader := &io.LimitedReader{R: res.Body, N: conf.MaxFetchLimit}
				scanner := bufio.NewScanner(limitedReader)
				twter := types.Twter{Nick: feed.Nick}
				if strings.HasPrefix(feed.URL, conf.BaseURL) {
					twter.URL = URLForUser(conf, feed.Nick)
					twter.Avatar = URLForAvatar(conf, feed.Nick)
				} else {
					twter.URL = feed.URL
					avatar := GetExternalAvatar(conf, feed.URL)
					if avatar != "" {
						twter.Avatar = URLForExternalAvatar(conf, feed.URL)
					}
				}
				twts, old, err := ParseFile(scanner, twter, conf.MaxCacheTTL, conf.MaxCacheItems)
				if err != nil {
					log.WithError(err).Errorf("error parsing feed %s", feed)
					twtsch <- nil
					return
				}
				log.Infof("fetched %d new and %d old twts from %s", len(twts), len(old), feed)

				// Archive old twts
				for _, twt := range old {
					if archive.Has(twt.Hash()) {
						// assume we have archived this twt and all older ones
						break
					}
					if err := archive.Archive(twt); err != nil {
						log.WithError(err).Errorf("error archiving twt %s aborting", twt.Hash())
						break
					}
					metrics.Counter("archive", "size").Inc()
				}

				lastmodified := res.Header.Get("Last-Modified")
				mu.Lock()
				cache[feed.URL] = Cached{Twts: twts, Lastmodified: lastmodified}
				mu.Unlock()
			case http.StatusNotModified: // 304
				log.Infof("feed %s has not changed", feed)
				mu.RLock()
				twts = cache[feed.URL].Twts
				mu.RUnlock()
			}

			twtsch <- twts
		}(feed)
	}

	// close twts channel when all goroutines are done
	go func() {
		wg.Wait()
		close(twtsch)
	}()

	for range twtsch {
	}

	metrics.Gauge("cache", "feeds").Set(float64(len(cache)))
	count := 0
	for _, cached := range cache {
		count += len(cached.Twts)
	}
	metrics.Gauge("cache", "twts").Set(float64(count))
}

// GetAll ...
func (cache Cache) GetAll() types.Twts {
	var alltwts types.Twts
	for _, cached := range cache {
		alltwts = append(alltwts, cached.Twts...)
	}
	return alltwts
}

// GetByPrefix ...
func (cache Cache) GetByPrefix(prefix string, refresh bool) types.Twts {
	key := fmt.Sprintf("prefix:%s", prefix)
	cached, ok := cache[key]
	if ok && !refresh {
		return cached.Twts
	}

	var twts types.Twts

	for url, cached := range cache {
		if strings.HasPrefix(url, prefix) {
			twts = append(twts, cached.Twts...)
		}
	}

	// FIXME: This is probably not thread safe :/
	cache[key] = Cached{Twts: twts, Lastmodified: time.Now().Format(time.RFC3339)}

	return twts
}

// IsCached ...

func (cache Cache) IsCached(url string) bool {
	_, ok := cache[url]
	return ok
}

// GetByURL ...
func (cache Cache) GetByURL(url string) types.Twts {
	if cached, ok := cache[url]; ok {
		return cached.Twts
	}
	return types.Twts{}
}

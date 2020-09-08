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

const (
	feedCacheFile = "cache"
)

// Cacheable ...
type Cacheable interface {
	IsZero() bool
	Hash() string
}

// Cacheables ...
type Cacheables []Cacheable

// CachedItems ...
type CachedItems struct {
	cache        map[string]Cacheable
	Items        Cacheables
	Lastmodified string
}

// Lookup ...
func (cached CachedItems) Lookup(hash string) (Cacheable, bool) {
	log.Debugf("CachedItems.Lookup(%s)", hash)

	item, ok := cached.cache[hash]
	log.Debugf(" item: %#v", item)
	if ok {
		return item, true
	}

	for _, item := range cached.Items {
		if item.Hash() == hash {
			if cached.cache == nil {
				cached.cache = make(map[string]Cacheable)
			}
			cached.cache[hash] = item
			log.Debugf(" item: %#v", item)
			return item, true
		}
	}

	return nil, false
}

// OldCache ...
type OldCache map[string]CachedItems

// Cache ...
type Cache struct {
	mu    sync.RWMutex
	Items map[string]CachedItems
}

// Store ...
func (cache *Cache) Store(path string) error {
	b := new(bytes.Buffer)
	enc := gob.NewEncoder(b)
	err := enc.Encode(cache)

	if err != nil {
		log.WithError(err).Error("error encoding cache")
		return err
	}

	f, err := os.OpenFile(filepath.Join(path, feedCacheFile), os.O_CREATE|os.O_WRONLY, 0666)
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

// LoadCache ...
func LoadCache(path string) (*Cache, error) {
	cache := &Cache{
		Items: make(map[string]CachedItems),
	}

	f, err := os.Open(filepath.Join(path, feedCacheFile))
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
		log.WithError(err).Error("error decoding cache (trying OldCache)")

		f.Seek(0, io.SeekStart)
		oldcache := make(OldCache)
		dec := gob.NewDecoder(f)
		err = dec.Decode(&oldcache)
		if err != nil {
			log.WithError(err).Error("error decoding cache")
			return nil, err
		}
		for url, cached := range oldcache {
			cache.mu.Lock()
			cache.Items[url] = cached
			cache.mu.Unlock()
		}
	}
	return cache, nil
}

const maxfetchers = 50

// FetchTwts ...
func (cache *Cache) FetchTwts(conf *Config, archive Archiver, feeds types.Feeds) {
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

			cache.mu.RLock()
			if cached, ok := cache.Items[feed.URL]; ok {
				if cached.Lastmodified != "" {
					headers.Set("If-Modified-Since", cached.Lastmodified)
				}
			}
			cache.mu.RUnlock()

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
					avatar := GetExternalAvatar(conf, feed.Nick, feed.URL)
					if avatar != "" {
						twter.Avatar = URLForExternalAvatar(conf, feed.Nick, feed.URL)
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
					if !archive.Has(twt.Hash()) {
						if err := archive.Archive(twt); err != nil {
							log.WithError(err).Errorf("error archiving twt %s aborting", twt.Hash())
							metrics.Counter("archive", "error").Inc()
						} else {
							metrics.Counter("archive", "size").Inc()
						}
					}
				}

				lastmodified := res.Header.Get("Last-Modified")
				cache.mu.Lock()
				cache.Items[feed.URL] = CachedItems{
					cache:        make(map[string]Cacheable),
					Items:        Cacheables{twts},
					Lastmodified: lastmodified,
				}
				cache.mu.Unlock()
			case http.StatusNotModified: // 304
				log.Infof("feed %s has not changed", feed)
				cache.mu.RLock()
				for _, item := range cache.Items[feed.URL].Items {
					twts = append(twts, item.(types.Twt))
				}
				cache.mu.RUnlock()
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

	cache.mu.RLock()
	metrics.Gauge("cache", "feeds").Set(float64(len(cache.Items)))
	count := 0
	for _, cached := range cache.Items {
		count += len(cached.Items)
	}
	cache.mu.RUnlock()
	metrics.Gauge("cache", "twts").Set(float64(count))
}

func (cache *Cache) LookupTwt(key string) (twt types.Twt, ok bool) {
	log.Debugf("LookupTwt(%s)", key)
	var item Cacheable
	item, ok = cache.LookupItem(key)
	log.Debugf(" item: %#v", item)
	if ok {
		twt = item.(types.Twt)
	}
	return
}

// LookupItem ...
func (cache *Cache) LookupItem(key string) (Cacheable, bool) {
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	for _, cached := range cache.Items {
		item, ok := cached.Lookup(key)
		if ok {
			return item, true
		}
	}
	return nil, false
}

func (cache *Cache) Count() int {
	var count int
	cache.mu.RLock()
	for _, cached := range cache.Items {
		count += len(cached.Items)
	}
	cache.mu.RUnlock()
	return count
}

func (cache *Cache) GetAllTwts() (twts types.Twts) {
	for _, item := range cache.GetAllItems() {
		twts = append(twts, item.(types.Twt))
	}
	return
}

// GetAllItems ...
func (cache *Cache) GetAllItems() (items Cacheables) {
	cache.mu.RLock()
	for _, cached := range cache.Items {
		items = append(items, cached.Items...)
	}
	cache.mu.RUnlock()
	return
}

func (cache *Cache) GetTwtsByPrefix(prefix string, refresh bool) (twts types.Twts) {
	for _, item := range cache.GetItemsByPrefix(prefix, refresh) {
		twts = append(twts, item.(types.Twts)...)
	}
	return
}

// GetItemsByPrefix ...
func (cache *Cache) GetItemsByPrefix(prefix string, refresh bool) Cacheables {
	key := fmt.Sprintf("prefix:%s", prefix)
	cache.mu.RLock()
	cached, ok := cache.Items[key]
	cache.mu.RUnlock()
	if ok && !refresh {
		return cached.Items
	}

	var items Cacheables

	cache.mu.RLock()
	for key, cached := range cache.Items {
		if strings.HasPrefix(key, prefix) {
			items = append(items, cached.Items...)
		}
	}
	cache.mu.RUnlock()

	cache.mu.Lock()
	cache.Items[key] = CachedItems{
		cache:        make(map[string]Cacheable),
		Items:        items,
		Lastmodified: time.Now().Format(time.RFC3339),
	}
	cache.mu.Unlock()

	return items
}

// IsCached ...
func (cache *Cache) IsCached(key string) bool {
	cache.mu.RLock()
	_, ok := cache.Items[key]
	cache.mu.RUnlock()
	return ok
}

func (cache *Cache) GetTwtsByURL(url string) (twts types.Twts) {
	for _, item := range cache.GetItemsByURL(url) {
		twts = append(twts, item.(types.Twts)...)
	}
	return
}

// GetByURL ...
func (cache *Cache) GetItemsByURL(url string) Cacheables {
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	if cached, ok := cache.Items[url]; ok {
		return cached.Items
	}
	return nil
}

// DeleteItems ...
func (cache *Cache) DeleteItems(feeds types.Feeds) {
	for feed := range feeds {
		cache.mu.Lock()
		delete(cache.Items, feed.URL)
		cache.mu.Unlock()
	}
}

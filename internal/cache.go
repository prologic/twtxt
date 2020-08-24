package internal

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"expvar"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/prologic/twtxt"
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
func (cache Cache) FetchTwts(conf *Config, sources map[string]string) {
	var mu sync.RWMutex

	stime := time.Now()
	defer func() {
		metrics.Gauge(
			"server",
			"feed_cache_last_processing_time_seconds",
		).Set(
			float64(time.Now().Sub(stime) / 1e9),
		)
	}()

	// buffered to let goroutines write without blocking before the main thread
	// begins reading
	twtsch := make(chan types.Twts, len(sources))

	var wg sync.WaitGroup
	// max parallel http fetchers
	var fetchers = make(chan struct{}, maxfetchers)

	for nick, url := range sources {
		stats.Add("fetchers", 1)
		wg.Add(1)
		fetchers <- struct{}{}
		// anon func takes needed variables as arg, avoiding capture of iterator variables
		go func(nick string, url string) {
			defer func() {
				<-fetchers
				wg.Done()
				stats.Add("fetchers", -1)
			}()

			req, err := http.NewRequest("GET", url, nil)
			if err != nil {
				log.WithError(err).Errorf("%s: http.NewRequest fail: %s", url, err)
				twtsch <- nil
				return
			}

			req.Header.Set("User-Agent", fmt.Sprintf("twtxt/%s", twtxt.FullVersion()))

			mu.RLock()
			if cached, ok := cache[url]; ok {
				if cached.Lastmodified != "" {
					req.Header.Set("If-Modified-Since", cached.Lastmodified)
				}
			}
			mu.RUnlock()

			client := http.Client{
				Timeout: time.Second * 15,
			}
			resp, err := client.Do(req)
			if err != nil {
				log.WithError(err).Errorf("%s: client.Do fail: %s", url, err)
				twtsch <- nil
				return
			}
			defer resp.Body.Close()

			actualurl := resp.Request.URL.String()
			if actualurl != url {
				log.WithError(err).Errorf("feed for %s changed from %s to %s", nick, url, actualurl)
				url = actualurl
			}

			if url == "" {
				log.WithField("nick", nick).WithField("url", url).Warn("empty url")
				twtsch <- nil
				return
			}

			var twts types.Twts

			switch resp.StatusCode {
			case http.StatusOK: // 200
				limitedReader := &io.LimitedReader{R: resp.Body, N: conf.MaxFetchLimit}
				scanner := bufio.NewScanner(limitedReader)
				twter := types.Twter{Nick: nick}
				if strings.HasPrefix(url, conf.BaseURL) {
					twter.URL = URLForUser(conf, nick)
					twter.Avatar = URLForAvatar(conf, nick)
				} else {
					twter.URL = url
					twter.Avatar = GetExternalAvatar(conf, url)
				}
				twts, err := ParseFile(scanner, twter)
				if err != nil {
					log.WithError(err).Errorf("error parsing feed %s: %s", nick, url)
					twtsch <- nil
					return
				}

				if len(twts) == 0 {
					log.WithField("nick", nick).WithField("url", url).Warn("no twts parsed, possibly bad feed")
					twtsch <- nil
					return
				}

				lastmodified := resp.Header.Get("Last-Modified")
				mu.Lock()
				cache[url] = Cached{Twts: twts, Lastmodified: lastmodified}
				mu.Unlock()
			case http.StatusNotModified: // 304
				mu.RLock()
				twts = cache[url].Twts
				mu.RUnlock()
			}

			twtsch <- twts
		}(nick, url)
	}

	// close twts channel when all goroutines are done
	go func() {
		wg.Wait()
		close(twtsch)
	}()

	for range twtsch {
	}

	expvar.Get("sources").(*expvar.Int).Set(int64(len(cache)))

	var count int64
	for _, cahced := range cache {
		count += int64(len(cahced.Twts))
	}
	expvar.Get("cached").(*expvar.Int).Set(count)
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

// GetByURL ...
func (cache Cache) GetByURL(url string) types.Twts {
	if cached, ok := cache[url]; ok {
		return cached.Twts
	}
	return types.Twts{}
}

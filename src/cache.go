package twtxt

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type Cached struct {
	Tweets       Tweets
	Lastmodified string
}

// key: url
type Cache map[string]Cached

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

func (cache Cache) FetchTweets(sources map[string]string) {
	var mu sync.RWMutex

	// buffered to let goroutines write without blocking before the main thread
	// begins reading
	tweetsch := make(chan Tweets, len(sources))

	var wg sync.WaitGroup
	// max parallel http fetchers
	var fetchers = make(chan struct{}, maxfetchers)

	for nick, url := range sources {
		wg.Add(1)
		fetchers <- struct{}{}
		// anon func takes needed variables as arg, avoiding capture of iterator variables
		go func(nick string, url string) {
			defer func() {
				<-fetchers
				wg.Done()
			}()

			req, err := http.NewRequest("GET", url, nil)
			if err != nil {
				log.WithError(err).Errorf("%s: http.NewRequest fail: %s", url, err)
				tweetsch <- nil
				return
			}

			req.Header.Set("User-Agent", fmt.Sprintf("twtxt/%s", FullVersion()))

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
				tweetsch <- nil
				return
			}
			defer resp.Body.Close()

			actualurl := resp.Request.URL.String()
			if actualurl != url {
				log.WithError(err).Errorf("feed for %s changed from %s to %s", nick, url, actualurl)
				url = actualurl
			}

			var tweets Tweets

			switch resp.StatusCode {
			case http.StatusOK: // 200
				scanner := bufio.NewScanner(resp.Body)
				tweets, err := ParseFile(scanner, Tweeter{Nick: nick, URL: url})
				if err != nil {
					log.WithError(err).Errorf("error parsing feed %s: %s", nick, url)
					tweetsch <- nil
					return
				}
				lastmodified := resp.Header.Get("Last-Modified")
				mu.Lock()
				cache[url] = Cached{Tweets: tweets, Lastmodified: lastmodified}
				mu.Unlock()
			case http.StatusNotModified: // 304
				mu.RLock()
				tweets = cache[url].Tweets
				mu.RUnlock()
			}

			tweetsch <- tweets
		}(nick, url)
	}

	// close tweets channel when all goroutines are done
	go func() {
		wg.Wait()
		close(tweetsch)
	}()

	for range tweetsch {
	}
}

func (cache Cache) GetAll() Tweets {
	var alltweets Tweets
	for _, cached := range cache {
		alltweets = append(alltweets, cached.Tweets...)
	}
	return alltweets
}

func (cache Cache) GetByURL(url string) Tweets {
	if cached, ok := cache[url]; ok {
		return cached.Tweets
	}
	return Tweets{}
}

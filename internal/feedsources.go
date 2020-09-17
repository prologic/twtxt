package internal

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
)

type FeedSource struct {
	Name string
	URL  string
}

type FeedSourceMap map[string][]FeedSource

type FeedSources struct {
	Sources FeedSourceMap `"json:sources"`
}

func SaveFeedSources(feedsources *FeedSources, path string) error {
	b := new(bytes.Buffer)
	enc := gob.NewEncoder(b)
	err := enc.Encode(feedsources)
	if err != nil {
		log.WithError(err).Error("error encoding feedsources ")
		return err
	}

	f, err := os.OpenFile(filepath.Join(path, "feedsources"), os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.WithError(err).Error("error opening feed sources file for writing")
		return err
	}

	defer f.Close()

	if _, err = f.Write(b.Bytes()); err != nil {
		log.WithError(err).Error("error writing feed sources file")
		return err
	}
	return nil
}

func LoadFeedSources(path string) (*FeedSources, error) {
	feedsources := &FeedSources{
		Sources: make(FeedSourceMap),
	}

	f, err := os.Open(filepath.Join(path, "feedsources"))
	if err != nil {
		if !os.IsNotExist(err) {
			log.WithError(err).Error("error loading feed sources, file not found")
			return nil, err
		}
		return feedsources, nil
	}
	defer f.Close()

	dec := gob.NewDecoder(f)
	err = dec.Decode(&feedsources)
	if err != nil {
		log.WithError(err).Error("error decoding feed sources")
		return nil, err
	}
	return feedsources, nil
}

func FetchFeedSources(conf *config, sources []string) *FeedSources {
	var (
		mu sync.RWMutex
		wg sync.WaitGroup
	)

	feedsources := &FeedSources{
		Sources: make(FeedSourceMap),
	}

	for _, url := range sources {
		wg.Add(1)
		// anon func takes needed variables as arg, avoiding capture of iterator variables
		go func(url string) {
			defer func() {
				wg.Done()
			}()

			res, err := Request(conf, http.MethodGet, url, nil)
			if err != nil {
				log.WithError(err).Errorf("error fetching feedsource %s", url)
				return
			}
			defer res.Body.Close()

			switch res.StatusCode {
			case http.StatusOK: // 200
				scanner := bufio.NewScanner(res.Body)
				fs, err := ParseFeedSource(scanner)
				if err != nil {
					log.WithError(err).Errorf("error parsing feed source: %s", url)
					return
				}

				mu.Lock()
				feedsources.Sources[url] = fs
				mu.Unlock()
			}
		}(url)
	}

	wg.Wait()

	return feedsources
}

func ParseFeedSource(scanner *bufio.Scanner) (feedsources []FeedSource, err error) {
	re := regexp.MustCompile(`^(.+?)(\s+)(.+)$`) // .+? is ungreedy
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		parts := re.FindStringSubmatch(line)
		// "Submatch 0 is the match of the entire expression, submatch 1 the
		// match of the first parenthesized subexpression, and so on."
		if len(parts) != 4 {
			log.Warnf("could not parse: '%s'", line)
			continue
		}
		feedsources = append(feedsources, FeedSource{parts[1], parts[3]})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return feedsources, nil
}

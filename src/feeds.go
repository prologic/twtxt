package twtxt

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type Feed struct {
	Name string
	URL  string
}

type Feeds []Feed

func SaveFeeds(feeds Feeds, path string) error {
	b := new(bytes.Buffer)
	enc := gob.NewEncoder(b)
	err := enc.Encode(feeds)
	if err != nil {
		log.WithError(err).Error("error encoding feeds ")
		return err
	}

	f, err := os.OpenFile(filepath.Join(path, "sources"), os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.WithError(err).Error("error opening feeds file for writing")
		return err
	}

	defer f.Close()

	if _, err = f.Write(b.Bytes()); err != nil {
		log.WithError(err).Error("error writing feeds file")
		return err
	}
	return nil
}

func LoadFeeds(path string) (Feeds, error) {
	var feeds Feeds

	f, err := os.Open(filepath.Join(path, "sources"))
	if err != nil {
		if !os.IsNotExist(err) {
			log.WithError(err).Error("error loading feeds, file not found")
			return nil, err
		}
		return feeds, nil
	}
	defer f.Close()

	dec := gob.NewDecoder(f)
	err = dec.Decode(&feeds)
	if err != nil {
		log.WithError(err).Error("error decoding feeds")
		return nil, err
	}
	return feeds, nil
}

func FetchFeeds(sources []string) Feeds {
	var (
		mu sync.RWMutex
		wg sync.WaitGroup

		feeds Feeds
	)

	for _, url := range sources {
		wg.Add(1)
		// anon func takes needed variables as arg, avoiding capture of iterator variables
		go func(url string) {
			defer func() {
				wg.Done()
			}()

			req, err := http.NewRequest("GET", url, nil)
			if err != nil {
				log.WithError(err).Errorf("%s: http.NewRequest fail: %s", url, err)
				return
			}

			req.Header.Set("User-Agent", fmt.Sprintf("twtxt/%s", FullVersion()))

			client := http.Client{
				Timeout: time.Second * 15,
			}
			resp, err := client.Do(req)
			if err != nil {
				log.WithError(err).Errorf("%s: client.Do fail: %s", url, err)
				return
			}
			defer resp.Body.Close()

			switch resp.StatusCode {
			case http.StatusOK: // 200
				scanner := bufio.NewScanner(resp.Body)
				fs, err := ParseFeedSource(scanner)
				if err != nil {
					log.WithError(err).Errorf("error parsing feed source: %s", url)
					return
				}

				mu.Lock()
				feeds = append(feeds, fs...)
				mu.Unlock()
			}
		}(url)
	}

	wg.Wait()

	return feeds
}

func ParseFeedSource(scanner *bufio.Scanner) (feeds Feeds, err error) {
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
		feeds = append(feeds, Feed{parts[1], parts[3]})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return feeds, nil
}

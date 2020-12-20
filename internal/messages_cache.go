package internal

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	log "github.com/sirupsen/logrus"
)

const (
	messagesCacheFile = "msgscache"
)

// MessagesCache ...
type MessagesCache struct {
	mu            sync.RWMutex
	MessageCounts map[string]int
}

// NewMessagesCache ...
func NewMessagesCache() *MessagesCache {
	return &MessagesCache{
		MessageCounts: make(map[string]int),
	}
}

// Store ...
func (cache *MessagesCache) Store(path string) error {
	b := new(bytes.Buffer)
	enc := gob.NewEncoder(b)
	err := enc.Encode(cache)
	if err != nil {
		log.WithError(err).Error("error encoding cache")
		return err
	}

	f, err := os.OpenFile(filepath.Join(path, messagesCacheFile), os.O_CREATE|os.O_WRONLY, 0666)
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

// LoadMessagesCache ...
func LoadMessagesCache(path string) (*MessagesCache, error) {
	cache := &MessagesCache{
		MessageCounts: make(map[string]int),
	}

	f, err := os.Open(filepath.Join(path, messagesCacheFile))
	if err != nil {
		if !os.IsNotExist(err) {
			log.WithError(err).Error("error loading messages cache, cache not found")
			return nil, err
		}
		return cache, nil
	}
	defer f.Close()

	dec := gob.NewDecoder(f)
	err = dec.Decode(&cache)
	if err != nil {
		log.WithError(err).Error("error decoding messages cache")
		return nil, err
	}
	return cache, nil
}

// Refresh ...
func (cache *MessagesCache) Refresh(conf *Config) {
	p := filepath.Join(conf.Data, msgsDir)
	if err := os.MkdirAll(p, 0755); err != nil {
		log.WithError(err).Error("error creating messages directory")
		return
	}

	err := filepath.Walk(p, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.WithError(err).Error("error walking messages directory")
			return err
		}

		count, err := countMessages(conf, info.Name())
		if err != nil {
			log.WithError(err).Error("error counting messages")
			return fmt.Errorf("error counting messages: %w", err)
		}
		cache.mu.Lock()
		cache.MessageCounts[info.Name()] = count
		cache.mu.Unlock()

		return nil
	})
	if err != nil {
		log.WithError(err).Errorf("error refreshing messages")
		return
	}

	if err := cache.Store(conf.Data); err != nil {
		log.WithError(err).Error("error saving blogs cache")
	}
}

// Inc ...
func (cache *MessagesCache) Inc(username string) {
	cache.mu.Lock()
	cache.MessageCounts[username]++
	cache.mu.Unlock()
}

// Dec ...
func (cache *MessagesCache) Dec(username string) {
	cache.mu.Lock()
	cache.MessageCounts[username]--
	cache.mu.Unlock()
}

// Get ...
func (cache *MessagesCache) Get(username string) int {
	cache.mu.RLock()
	count := cache.MessageCounts[username]
	cache.mu.RUnlock()
	return count
}

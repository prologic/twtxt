package internal

import (
	"bytes"
	"encoding/gob"
	"io"
	"os"
	"path/filepath"
	"sync"

	log "github.com/sirupsen/logrus"
)

const (
	blogsCacheFile = "blogscache"
)

// OldBlogsCache ...
type OldBlogsCache map[string]*BlogPost

// BlogsCache ...
type BlogsCache struct {
	mu    sync.RWMutex
	Blogs map[string]*BlogPost
}

// NewBlogsCache ...
func NewBlogsCache() *BlogsCache {
	return &BlogsCache{
		Blogs: make(map[string]*BlogPost),
	}
}

// Store ...
func (cache BlogsCache) Store(path string) error {
	b := new(bytes.Buffer)
	enc := gob.NewEncoder(b)
	err := enc.Encode(cache)
	if err != nil {
		log.WithError(err).Error("error encoding cache")
		return err
	}

	f, err := os.OpenFile(filepath.Join(path, blogsCacheFile), os.O_CREATE|os.O_WRONLY, 0666)
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

// LoadBlogsCache ...
func LoadBlogsCache(path string) (*BlogsCache, error) {
	cache := &BlogsCache{
		Blogs: make(map[string]*BlogPost),
	}

	f, err := os.Open(filepath.Join(path, blogsCacheFile))
	if err != nil {
		if !os.IsNotExist(err) {
			log.WithError(err).Error("error loading blogs cache, cache not found")
			return nil, err
		}
		return cache, nil
	}
	defer f.Close()

	dec := gob.NewDecoder(f)
	err = dec.Decode(&cache)
	if err != nil {
		log.WithError(err).Error("error decoding blogs cache (trying OldBlogsCache)")

		f.Seek(0, io.SeekStart)
		oldcache := make(OldBlogsCache)
		dec := gob.NewDecoder(f)
		err = dec.Decode(&oldcache)
		if err != nil {
			log.WithError(err).Error("error decoding cache")
			return nil, err
		}
		for hash, blogPost := range oldcache {
			cache.Blogs[hash] = blogPost
		}
	}
	return cache, nil
}

// UpdateBlogs ...
func (cache BlogsCache) UpdateBlogs(conf *config) {
	blogPosts, err := GetAllBlogPosts(conf)
	if err != nil {
		log.WithError(err).Error("error getting all blog posts")
		return
	}

	for _, blogPost := range blogPosts {
		cache.Add(blogPost)
	}

	if err := cache.Store(conf.data); err != nil {
		log.WithError(err).Error("error saving blogs cache")
	}
}

// Add ...
func (cache BlogsCache) Add(blogPost *BlogPost) {
	cache.mu.Lock()
	cache.Blogs[blogPost.Hash()] = blogPost
	cache.mu.Unlock()
}

// Get ...
func (cache BlogsCache) Get(hash string) (*BlogPost, bool) {
	cache.mu.RLock()
	blogPost, ok := cache.Blogs[hash]
	cache.mu.RUnlock()
	return blogPost, ok
}

// Count ...
func (cache BlogsCache) Count() int {
	return len(cache.Blogs)
}

// GetAll ...
func (cache BlogsCache) GetAll() BlogPosts {
	var blogPosts BlogPosts
	cache.mu.RLock()
	for _, blogPost := range cache.Blogs {
		blogPosts = append(blogPosts, blogPost)
	}
	cache.mu.RUnlock()
	return blogPosts
}

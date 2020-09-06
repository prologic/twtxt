package internal

import (
	"bytes"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
	"github.com/writeas/slug"
	"golang.org/x/crypto/blake2b"
)

const (
	blogsDir       = "blogs"
	blogHashLength = 7
)

var (
	ErrInvalidBlogPath = errors.New("error: invalid blog path")
)

type BlogPost struct {
	Author string `json:"author"`
	Year   int    `json:"year"`
	Month  int    `json:"month"`
	Date   int    `json:"date"`
	Slug   string `json:"slub"`
	Title  string `json:"title"`
	Twt    string `json:"twt"`

	hash string
	data *bytes.Buffer
}

type BlogPosts []*BlogPost

func (bs BlogPosts) Len() int {
	return len(bs)
}
func (bs BlogPosts) Less(i, j int) bool {
	return bs[i].Created().After(bs[j].Created())
}
func (bs BlogPosts) Swap(i, j int) {
	bs[i], bs[j] = bs[j], bs[i]
}

func GetBlogPostsByAuthor(conf *Config, author string) (BlogPosts, error) {
	var blogPosts BlogPosts

	p := filepath.Join(conf.Data, blogsDir, author)
	if err := os.MkdirAll(p, 0755); err != nil {
		log.WithError(err).Error("error creating blogs directory")
		return nil, err
	}

	err := filepath.Walk(p, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.WithError(err).Error("error walking blog directory")
			return err
		}

		if !info.IsDir() && filepath.Ext(info.Name()) == ".md" {
			blogPost, err := BlogPostFromFile(conf, path)
			if err != nil {
				log.WithError(err).Errorf("error loading blog post %s", path)
				return err
			}
			blogPosts = append(blogPosts, blogPost)
		}
		return nil
	})
	if err != nil {
		log.WithError(err).Errorf("error listing blog posts for %s", author)
		return nil, err
	}

	return blogPosts, nil
}

func GetAllBlogPosts(conf *Config) (BlogPosts, error) {
	var blogPosts BlogPosts

	p := filepath.Join(conf.Data, blogsDir)
	if err := os.MkdirAll(p, 0755); err != nil {
		log.WithError(err).Error("error creating blogs directory")
		return nil, err
	}

	err := filepath.Walk(p, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.WithError(err).Error("error walking blog directory")
			return err
		}

		if !info.IsDir() && filepath.Ext(info.Name()) == ".md" {
			blogPost, err := BlogPostFromFile(conf, path)
			if err != nil {
				log.WithError(err).Errorf("error loading blog post %s", path)
				return err
			}
			blogPosts = append(blogPosts, blogPost)
		}
		return nil
	})
	if err != nil {
		log.WithError(err).Errorf("error listing all blog posts")
		return nil, err
	}

	return blogPosts, nil
}

func NewBlogPost(author, title string) *BlogPost {
	now := time.Now()

	b := &BlogPost{
		Author: author,
		Year:   now.Year(),
		Month:  int(now.Month()),
		Date:   now.Day(),

		Title: title,
		Slug:  slug.Make(title),

		data: &bytes.Buffer{},
	}

	return b
}

func BlogPostFromFile(conf *Config, fn string) (*BlogPost, error) {
	p := filepath.Join(conf.Data, blogsDir)
	if err := os.MkdirAll(p, 0755); err != nil {
		log.WithError(err).Error("error creating blogs directory")
		return nil, err
	}

	rel, err := filepath.Rel(p, fn)
	if err != nil {
		log.WithError(err).Error("filepath.Rel() error")
		return nil, err
	}

	parts := strings.Split(rel, "/")
	if len(parts) != 5 {
		log.WithField("fn", fn).Errorf("invalid blog path expected 5 tokens got %d", len(parts))
		return nil, ErrInvalidBlogPath
	}

	author := parts[0]
	year := SafeParseInt(parts[1], 1970)
	month := SafeParseInt(parts[2], 1)
	date := SafeParseInt(parts[3], 1)
	slug := strings.TrimSuffix(parts[4], filepath.Ext(parts[4]))

	b := &BlogPost{
		Author: author,
		Year:   year,
		Month:  month,
		Date:   date,

		Slug: slug,

		data: &bytes.Buffer{},
	}

	if err := b.LoadMetadata(conf); err != nil {
		return nil, err
	}

	return b, nil
}

func BlogPostFromParams(conf *Config, p httprouter.Params) (*BlogPost, error) {
	author := p.ByName("author")
	year := SafeParseInt(p.ByName("year"), 1970)
	month := SafeParseInt(p.ByName("month"), 1)
	date := SafeParseInt(p.ByName("date"), 1)
	slug := p.ByName("slug")

	b := &BlogPost{
		Author: author,
		Year:   year,
		Month:  month,
		Date:   date,

		Slug: slug,

		data: &bytes.Buffer{},
	}

	if err := b.LoadMetadata(conf); err != nil {
		log.WithError(err).Errorf("error loading metdata for blog post %s", b)
		return nil, err
	}

	if err := b.Load(conf); err != nil {
		log.WithError(err).Errorf("error loading content for blog post %s", b)
		return nil, err
	}

	return b, nil
}

func (b *BlogPost) Created() time.Time {
	now := time.Now()
	return time.Date(b.Year, time.Month(b.Month), b.Date, 0, 0, 0, 0, now.Location())
}

func (b *BlogPost) Filename(ext string) string {
	fn := filepath.Join(
		b.Author,
		fmt.Sprintf("%04d", b.Year),
		fmt.Sprintf("%02d", b.Month),
		fmt.Sprintf("%02d", b.Date),
		fmt.Sprintf("%s%s", b.Slug, ext),
	)
	return fn
}

func (b *BlogPost) Reset() {
	b.data.Reset()
}

func (b *BlogPost) Write(p []byte) (int, error) {
	return b.data.Write(p)
}

func (b *BlogPost) WriteString(s string) (int, error) {
	return b.data.WriteString(s)
}

func (b *BlogPost) Bytes() []byte {
	return b.data.Bytes()
}

func (b *BlogPost) LoadMetadata(conf *Config) error {
	fn := filepath.Join(conf.Data, blogsDir, b.Filename(".json"))
	metadata, err := ioutil.ReadFile(fn)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(metadata, &b); err != nil {
		return err
	}

	return nil
}

func (b *BlogPost) Load(conf *Config) error {
	fn := filepath.Join(conf.Data, blogsDir, b.Filename(".md"))
	data, err := ioutil.ReadFile(fn)
	if err != nil {
		return err
	}

	b.data.Write(data)

	return nil
}

func (b *BlogPost) Save(conf *Config) error {
	data, err := json.Marshal(&b)
	if err != nil {
		return err
	}

	fn := filepath.Join(conf.Data, blogsDir, b.Filename(".json"))
	if err := os.MkdirAll(filepath.Dir(fn), 0755); err != nil {
		return err
	}
	if err := ioutil.WriteFile(fn, data, 0644); err != nil {
		return err
	}

	fn = filepath.Join(conf.Data, blogsDir, b.Filename(".md"))
	if err := os.MkdirAll(filepath.Dir(fn), 0755); err != nil {
		return err
	}

	if err := ioutil.WriteFile(fn, b.Bytes(), 0644); err != nil {
		return err
	}

	return nil
}

func (b *BlogPost) URL(baseURL string) string {
	return fmt.Sprintf(
		"%s/blog/%s",
		strings.TrimSuffix(baseURL, "/"),
		b.String(),
	)
}

func (b *BlogPost) Hash() string {
	if b.hash != "" {
		return b.hash
	}

	sum := blake2b.Sum256([]byte(b.String()))

	// Base32 is URL-safe, unlike Base64, and shorter than hex.
	encoding := base32.StdEncoding.WithPadding(base32.NoPadding)
	hash := strings.ToLower(encoding.EncodeToString(sum[:]))
	b.hash = hash[len(hash)-blogHashLength:]

	return b.hash
}

func (b *BlogPost) Content() string {
	return string(b.Bytes())
}

func (b *BlogPost) String() string {
	return fmt.Sprintf("%s/%04d/%02d/%02d/%s", b.Author, b.Year, b.Month, b.Date, b.Slug)
}

func WriteBlog(conf *Config, user *User, title, content string) (*BlogPost, error) {
	p := filepath.Join(conf.Data, blogsDir)
	if err := os.MkdirAll(p, 0755); err != nil {
		log.WithError(err).Error("error creating blogs directory")
		return nil, err
	}

	// Cleanup content
	content = strings.TrimSpace(content)
	content = strings.ReplaceAll(content, "\r\n", "\n")

	b := NewBlogPost(user.Username, title)
	if _, err := b.WriteString(content); err != nil {
		log.WithError(err).Error("error writing blog content")
		return nil, err
	}

	if err := b.Save(conf); err != nil {
		log.WithError(err).Error("error writing blog file")
		return nil, err
	}

	return b, nil
}

func WriteBlogAs(conf *Config, feed string, title, content string) (*BlogPost, error) {
	user := &User{Username: feed}
	return WriteBlog(conf, user, title, content)
}

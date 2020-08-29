package internal

import (
	"bytes"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/apex/log"
	"github.com/julienschmidt/httprouter"
	"github.com/writeas/slug"
	"golang.org/x/crypto/blake2b"
)

const (
	blogsDir       = "blogs"
	blogHashLength = 7
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

	if err := b.Load(conf); err != nil {
		return nil, err
	}

	return b, nil
}

func (b *BlogPost) Filename(ext string) string {
	fn := filepath.Join(
		b.Author,
		fmt.Sprintf("%04d", b.Year),
		fmt.Sprintf("%02%d", b.Month),
		fmt.Sprintf("%02%d", b.Date),
		fmt.Sprintf("%s%s", b.Slug, ext),
	)
	return fn
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

func (b *BlogPost) Load(conf *Config) error {
	fn := filepath.Join(conf.Data, blogsDir, b.Filename(".json"))
	metadata, err := ioutil.ReadFile(fn)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(metadata, &b); err != nil {
		return err
	}

	fn = filepath.Join(conf.Data, blogsDir, b.Filename(".md"))
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
	b.WriteString(content)

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

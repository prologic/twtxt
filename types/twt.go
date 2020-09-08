package types

import (
	"encoding/base32"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/apex/log"
	"github.com/writeas/slug"
	"golang.org/x/crypto/blake2b"
)

const (
	HashLength = 7
)

// Twter ...
type Twter struct {
	Nick    string
	URL     string
	Avatar  string
	Tagline string

	hash string
}

// Hash ...
func (twter Twter) Hash() string {
	if twter.hash != "" {
		return twter.hash
	}

	u, err := url.Parse(twter.URL)
	if err != nil {
		log.WithError(err).Warnf("Twter.Hash(): error parsing url: %s", twter.URL)
		return ""
	}

	s := slug.Make(fmt.Sprintf("%s/%s", u.Hostname(), u.Path))

	payload := fmt.Sprintf("%s/%s", s, twter.Nick)
	sum := blake2b.Sum256([]byte(payload))

	// Base32 is URL-safe, unlike Base64, and shorter than hex.
	encoding := base32.StdEncoding.WithPadding(base32.NoPadding)
	hash := strings.ToLower(encoding.EncodeToString(sum[:]))
	twter.hash = hash[len(hash)-HashLength:]

	return twter.hash
}

func (twter Twter) IsZero() bool {
	return twter.Nick == "" && twter.URL == ""
}

// Twt ...
type Twt struct {
	Twter   Twter
	Text    string
	Created time.Time

	hash string
}

// Mentions ...
func (twt Twt) Mentions() []Twter {
	var mentions []Twter

	seen := make(map[Twter]bool)
	re := regexp.MustCompile(`@<(.*?) (.*?)>`)
	matches := re.FindAllStringSubmatch(twt.Text, -1)
	for _, match := range matches {
		mention := Twter{Nick: match[1], URL: match[2]}
		if !seen[mention] {
			mentions = append(mentions, mention)
			seen[mention] = true
		}
	}

	return mentions
}

// Tags ...
func (twt Twt) Tags() []string {
	var tags []string

	seen := make(map[string]bool)
	re := regexp.MustCompile(`#<(.*?) .*?>`)
	matches := re.FindAllStringSubmatch(twt.Text, -1)
	for _, match := range matches {
		tag := match[1]
		if !seen[tag] {
			tags = append(tags, tag)
			seen[tag] = true
		}
	}

	return tags
}

// Subject ...
func (twt Twt) Subject() string {
	re := regexp.MustCompile(`^(@<.*>[, ]*)*(\(.*?\))(.*)`)
	match := re.FindStringSubmatch(twt.Text)
	if match != nil {
		return match[2]
	}
	// By default the subject is the Twt's Hash being replied to.
	return fmt.Sprintf("(#%s)", twt.Hash())
}

// Hash ...
func (twt Twt) Hash() string {
	if twt.hash != "" {
		return twt.hash
	}

	payload := twt.Twter.URL + "\n" + twt.Created.String() + "\n" + twt.Text
	sum := blake2b.Sum256([]byte(payload))

	// Base32 is URL-safe, unlike Base64, and shorter than hex.
	encoding := base32.StdEncoding.WithPadding(base32.NoPadding)
	hash := strings.ToLower(encoding.EncodeToString(sum[:]))
	twt.hash = hash[len(hash)-HashLength:]

	return twt.hash
}

func (twt Twt) IsZero() bool {
	return twt.Twter.IsZero() && twt.Created.IsZero() && twt.Text == ""
}

// Twts typedef to be able to attach sort methods
type Twts []Twt

func (twts Twts) IsZero() bool {
	return twts == nil || len(twts) == 0
}

func (twts Twts) Hash() string {
	if len(twts) > 0 {
		return twts[0].Hash()
	}
	return ""
}

func (twts Twts) Len() int {
	return len(twts)
}
func (twts Twts) Less(i, j int) bool {
	return twts[i].Created.After(twts[j].Created)
}
func (twts Twts) Swap(i, j int) {
	twts[i], twts[j] = twts[j], twts[i]
}

// Tags ...
func (twts Twts) Tags() map[string]int {
	tags := make(map[string]int)
	re := regexp.MustCompile(`#[-\w]+`)
	for _, twt := range twts {
		for _, tag := range re.FindAllString(twt.Text, -1) {
			tags[strings.TrimLeft(tag, "#")]++
		}
	}
	return tags
}

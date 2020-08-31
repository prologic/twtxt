package types

import (
	"encoding/base32"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/blake2b"
)

const (
	HashLength = 11
)

// Twter ...
type Twter struct {
	Nick    string
	URL     string
	Avatar  string
	Tagline string
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
	return ""
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

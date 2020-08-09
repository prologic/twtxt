package types

import (
	"encoding/base32"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/blake2b"
)

// Twter ...
type Twter struct {
	Nick string
	URL  string
}

// Twt ...
type Twt struct {
	Twter   Twter
	Text    string
	Created time.Time

	hash string
}

// Mentions ...
func (twt Twt) Mentions() []string {
	var mentions []string

	re := regexp.MustCompile(`@<(.*?) .*?>`)
	matches := re.FindAllStringSubmatch(twt.Text, -1)
	for _, match := range matches {
		mentions = append(mentions, match[1])
	}

	return mentions
}

// Tags ...
func (twt Twt) Tags() []string {
	var tags []string

	re := regexp.MustCompile(`#<(.*?) .*?>`)
	matches := re.FindAllStringSubmatch(twt.Text, -1)
	for _, match := range matches {
		tags = append(tags, match[1])
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

	payload := twt.Created.String() + "\n" + twt.Text
	sum := blake2b.Sum256([]byte(payload))

	// Base32 is URL-safe, unlike Base64, and shorter than hex.
	encoding := base32.StdEncoding.WithPadding(base32.NoPadding)
	twt.hash = strings.ToLower(encoding.EncodeToString(sum[:]))

	return twt.hash
}

// Twts typedef to be able to attach sort methods
type Twts []Twt

func (twts Twts) Len() int {
	return len(twts)
}
func (twts Twts) Less(i, j int) bool {
	return twts[i].Created.Before(twts[j].Created)
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

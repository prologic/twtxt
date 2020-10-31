package types

import (
	"encoding/base32"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/blake2b"
)

const (
	TwtHashLength = 7
)

var (
	tagsRe    = regexp.MustCompile(`#([-\w]+)`)
	subjectRe = regexp.MustCompile(`^(@<.*>[, ]*)*(\(.*?\))(.*)`)

	uriTagsRe     = regexp.MustCompile(`#<(.*?) .*?>`)
	uriMentionsRe = regexp.MustCompile(`@<(.*?) (.*?)>`)
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

func (twter Twter) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Nick    string `json:"nick"`
		URL     string `json:"url"`
		Avatar  string `json:"avatar"`
		Tagline string `json:"tagline"`
	}{
		Nick:    twter.Nick,
		URL:     twter.URL,
		Avatar:  twter.Avatar,
		Tagline: twter.Tagline,
	})
}

// Twt ...
type Twt struct {
	Twter        Twter
	Text         string
	MarkdownText string
	Created      time.Time

	hash string
}

func (twt Twt) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Twter        Twter     `json:"twter"`
		Text         string    `json:"text"`
		Created      time.Time `json:"created"`
		MarkdownText string    `json:"markdownText"`

		// Dynamic Fields
		Hash    string   `json:"hash"`
		Tags    []string `json:"tags"`
		Subject string   `json:"subject"`
	}{
		Twter:        twt.Twter,
		Text:         twt.Text,
		Created:      twt.Created,
		MarkdownText: twt.MarkdownText,

		// Dynamic Fields
		Hash:    twt.Hash(),
		Tags:    twt.Tags(),
		Subject: twt.Subject(),
	})
}

// Mentions ...
func (twt Twt) Mentions() []Twter {
	var mentions []Twter

	seen := make(map[Twter]bool)
	matches := uriMentionsRe.FindAllStringSubmatch(twt.Text, -1)
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

	matches := tagsRe.FindAllStringSubmatch(twt.Text, -1)
	matches = append(matches, uriTagsRe.FindAllStringSubmatch(twt.Text, -1)...)

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
	match := subjectRe.FindStringSubmatch(twt.Text)
	if match != nil {
		matchingSubject := match[2]
		matchedURITags := uriTagsRe.FindAllStringSubmatch(matchingSubject, -1)
		if matchedURITags != nil {
			// Re-add the (#xxx) back as the output
			return fmt.Sprintf("(#%s)", matchedURITags[0][1])
		}
		return matchingSubject
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
	twt.hash = hash[len(hash)-TwtHashLength:]

	return twt.hash
}

func (twt Twt) IsZero() bool {
	return twt.Twter.IsZero() && twt.Created.IsZero() && twt.Text == ""
}

// TwtMap ...
type TwtMap map[string]Twt

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
	for _, twt := range twts {
		for _, tag := range tagsRe.FindAllString(twt.Text, -1) {
			tags[strings.TrimLeft(tag, "#")]++
		}
	}
	return tags
}

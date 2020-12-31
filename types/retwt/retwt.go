package retwt

import (
	"bufio"
	"encoding/base32"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/apex/log"
	"github.com/jointwt/twtxt/types"
	"golang.org/x/crypto/blake2b"
)

func init() {
	gob.Register(&reTwt{})
}

var (
	tagsRe    = regexp.MustCompile(`#([-\w]+)`)
	subjectRe = regexp.MustCompile(`^(@<.*>[, ]*)*(\(.*?\))(.*)`)

	uriTagsRe     = regexp.MustCompile(`#<(.*?) .*?>`)
	uriMentionsRe = regexp.MustCompile(`@<(.*?) (.*?)>`)
)

type reTwt struct {
	twter   types.Twter
	text    string
	created time.Time

	hash     string
	mentions []types.TwtMention
	tags     []types.TwtTag
}

var _ types.Twt = (*reTwt)(nil)
var _ gob.GobEncoder = (*reTwt)(nil)
var _ gob.GobDecoder = (*reTwt)(nil)

func (twt reTwt) Links() types.LinkList { return nil }
func (twt reTwt) GobEncode() ([]byte, error) {
	enc := struct {
		Twter   types.Twter `json:"twter"`
		Text    string      `json:"text"`
		Created time.Time   `json:"created"`
		Hash    string      `json:"hash"`
	}{twt.twter, twt.text, twt.created, twt.hash}

	if twt.text == "" {
		return nil, fmt.Errorf("empty twt: %v", twt)
	}
	return json.Marshal(enc)
}
func (twt *reTwt) GobDecode(data []byte) error {
	enc := struct {
		Twter   types.Twter `json:"twter"`
		Text    string      `json:"text"`
		Created time.Time   `json:"created"`
		Hash    string      `json:"hash"`
	}{}
	err := json.Unmarshal(data, &enc)

	twt.twter = enc.Twter
	twt.text = enc.Text
	twt.created = enc.Created
	twt.hash = enc.Hash

	return err
}

func (twt reTwt) String() string {
	return fmt.Sprintf("%v\t%v", twt.created.Format(time.RFC3339), twt.text)
}

func NewReTwt(twter types.Twter, text string, created time.Time) *reTwt {
	return &reTwt{twter: twter, text: text, created: created}
}

func DecodeJSON(data []byte) (types.Twt, error) {
	twt := reTwt{}
	if err := twt.GobDecode(data); err != nil {
		return types.NilTwt, err
	}
	return twt, nil
}

func ParseLine(line string, twter types.Twter) (twt types.Twt, err error) {
	twt = types.NilTwt

	if line == "" {
		return
	}
	if strings.HasPrefix(line, "#") {
		return
	}

	re := regexp.MustCompile(`^(.+?)(\s+)(.+)$`) // .+? is ungreedy
	parts := re.FindStringSubmatch(line)
	// "Submatch 0 is the match of the entire expression, submatch 1 the
	// match of the first parenthesized subexpression, and so on."
	if len(parts) != 4 {
		err = ErrInvalidTwtLine
		return
	}

	created, err := ParseTime(parts[1])
	if err != nil {
		err = ErrInvalidTwtLine
		return
	}

	text := parts[3]

	twt = &reTwt{twter: twter, created: created, text: text}

	return
}

func ParseFile(r io.Reader, twter types.Twter) (*retwtFile, error) {
	scanner := bufio.NewScanner(r)

	nLines, nErrors := 0, 0

	f := &retwtFile{twter: twter}

	for scanner.Scan() {
		line := scanner.Text()
		nLines++

		twt, err := ParseLine(line, twter)
		if err != nil {
			nErrors++
			continue
		}

		if twt.IsZero() {
			continue
		}

		f.twts = append(f.twts, twt)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if (nLines+nErrors > 0) && nLines == nErrors {
		log.Warnf("erroneous feed dtected (nLines + nErrors > 0 && nLines == nErrors): %d/%d", nLines, nErrors)
		return nil, ErrInvalidFeed
	}

	return f, nil
}

func (twt reTwt) Twter() types.Twter { return twt.twter }
func (twt reTwt) Text() string       { return twt.text }
func (twt reTwt) MarkdownText() string {
	// we assume FmtOpts is always null for markdown.
	return formatMentionsAndTags(nil, twt.text, types.MarkdownFmt)
}
func (twt reTwt) FormatText(textFmt types.TwtTextFormat, fmtOpts types.FmtOpts) string {
	return formatMentionsAndTags(fmtOpts, twt.text, textFmt)
}

// func (twt *reTwt) SetFmtOpts(opts types.FmtOpts) { twt.fmtOpts = opts }
func (twt reTwt) Created() time.Time { return twt.created }
func (twt reTwt) MarshalJSON() ([]byte, error) {
	var tags types.TagList = twt.Tags()
	return json.Marshal(struct {
		Twter        types.Twter `json:"twter"`
		Text         string      `json:"text"`
		Created      time.Time   `json:"created"`
		MarkdownText string      `json:"markdownText"`

		// Dynamic Fields
		Hash    string   `json:"hash"`
		Tags    []string `json:"tags"`
		Subject string   `json:"subject"`
	}{
		Twter:        twt.Twter(),
		Text:         twt.Text(),
		Created:      twt.Created(),
		MarkdownText: twt.MarkdownText(),

		// Dynamic Fields
		Hash:    twt.Hash(),
		Tags:    tags.Tags(),
		Subject: twt.Subject().Text(),
	})
}

// Mentions ...
func (twt reTwt) Mentions() types.MentionList {
	if twt.mentions != nil {
		return twt.mentions
	}

	seen := make(map[string]struct{})
	matches := uriMentionsRe.FindAllStringSubmatch(twt.text, -1)
	for _, match := range matches {
		twter := types.Twter{Nick: match[1], URL: match[2]}
		if _, ok := seen[twter.URL]; !ok {
			twt.mentions = append(twt.mentions, &reMention{twter})
			seen[twter.URL] = struct{}{}
		}
	}

	return twt.mentions
}

// Tags ...
func (twt reTwt) Tags() types.TagList {
	if twt.tags != nil {
		return twt.tags
	}

	seen := make(map[string]struct{})

	matches := tagsRe.FindAllStringSubmatch(twt.text, -1)
	matches = append(matches, uriTagsRe.FindAllStringSubmatch(twt.text, -1)...)

	for _, match := range matches {
		tag := match[1]
		if _, ok := seen[tag]; !ok {
			twt.tags = append(twt.tags, &reTag{tag})
			seen[tag] = struct{}{}
		}
	}

	return twt.tags
}

// Subject ...
func (twt reTwt) Subject() types.Subject {
	match := subjectRe.FindStringSubmatch(twt.text)
	if match != nil {
		matchingSubject := match[2]
		matchedURITags := uriTagsRe.FindAllStringSubmatch(matchingSubject, -1)
		if matchedURITags != nil {
			// Re-add the (#xxx) back as the output
			return reSubject(fmt.Sprintf("(#%s)", matchedURITags[0][1]))
		}
		return reSubject(matchingSubject)
	}

	// By default the subject is the Twt's Hash being replied to.
	return reSubject(fmt.Sprintf("(#%s)", twt.Hash()))
}

// Hash ...
func (twt reTwt) Hash() string {
	if twt.hash != "" {
		return twt.hash
	}

	payload := twt.Twter().URL + "\n" + twt.Created().Format(time.RFC3339) + "\n" + twt.Text()
	sum := blake2b.Sum256([]byte(payload))

	// Base32 is URL-safe, unlike Base64, and shorter than hex.
	encoding := base32.StdEncoding.WithPadding(base32.NoPadding)
	hash := strings.ToLower(encoding.EncodeToString(sum[:]))
	twt.hash = hash[len(hash)-types.TwtHashLength:]

	return twt.hash
}

func (twt reTwt) IsZero() bool {
	return twt.Twter().IsZero() && twt.Created().IsZero() && twt.Text() == ""
}

type reMention struct {
	twter types.Twter
}

var _ types.TwtMention = (*reMention)(nil)

func (m *reMention) Twter() types.Twter { return m.twter }

type reTag struct {
	tag string
}

var _ types.TwtTag = (*reTag)(nil)

func (t *reTag) Tag() string {
	if t == nil {
		return ""
	}
	return t.tag
}

func (t *reTag) Text() string {
	sp := strings.Fields(t.tag)

	return sp[0]
}
func (t *reTag) Target() string {
	sp := strings.Fields(t.tag)
	if len(sp) > 1 {
		return sp[1]
	}
	return ""
}

// FormatMentionsAndTags turns `@<nick URL>` into `<a href="URL">@nick</a>`
// and `#<tag URL>` into `<a href="URL">#tag</a>` and a `!<hash URL>`
// into a `<a href="URL">!hash</a>`.
func formatMentionsAndTags(opts types.FmtOpts, text string, format types.TwtTextFormat) string {
	re := regexp.MustCompile(`(@|#)<([^ ]+) *([^>]+)>`)
	return re.ReplaceAllStringFunc(text, func(match string) string {
		parts := re.FindStringSubmatch(match)
		prefix, nick, url := parts[1], parts[2], parts[3]

		if format == types.TextFmt {
			switch prefix {
			case "@":
				if opts.IsLocalURL(url) && strings.HasSuffix(url, "/twtxt.txt") {
					return fmt.Sprintf("%s@%s", nick, opts.LocalURL().Hostname())
				}
				return fmt.Sprintf("@%s", nick)
			default:
				return fmt.Sprintf("%s%s", prefix, nick)
			}
		}

		if format == types.HTMLFmt {
			switch prefix {
			case "@":
				if opts.IsLocalURL(url) && strings.HasSuffix(url, "/twtxt.txt") {
					return fmt.Sprintf(`<a href="%s">@%s</a>`, opts.UserURL(url), nick)
				}
				return fmt.Sprintf(`<a href="%s">@%s</a>`, opts.ExternalURL(nick, url), nick)
			default:
				return fmt.Sprintf(`<a href="%s">%s%s</a>`, url, prefix, nick)
			}
		}

		switch prefix {
		case "@":
			// Using (#) anchors to add the nick to URL for now. The Fluter app needs it since
			// 	the Markdown plugin doesn't include the link text that contains the nick in its onTap callback
			// https://github.com/flutter/flutter_markdown/issues/286
			return fmt.Sprintf(`[@%s](%s#%s)`, nick, url, nick)
		default:
			return fmt.Sprintf(`[%s%s](%s)`, prefix, nick, url)
		}
	})
}

// FormatMentionsAndTagsForSubject turns `@<nick URL>` into `@nick`
func FormatMentionsAndTagsForSubject(text string) string {
	re := regexp.MustCompile(`(@|#)<([^ ]+) *([^>]+)>`)
	return re.ReplaceAllStringFunc(text, func(match string) string {
		parts := re.FindStringSubmatch(match)
		prefix, nick := parts[1], parts[2]
		return fmt.Sprintf(`%s%s`, prefix, nick)
	})
}

func ParseTime(timestr string) (tm time.Time, err error) {
	// Twtxt clients generally uses basically time.RFC3339Nano, but sometimes
	// there's a colon in the timezone, or no timezone at all.
	for _, layout := range []string{
		"2006-01-02T15:04:05.999999999Z07:00",
		"2006-01-02T15:04:05.999999999Z0700",
		"2006-01-02T15:04:05.999999999",
		"2006-01-02T15:04.999999999Z07:00",
		"2006-01-02T15:04.999999999Z0700",
		"2006-01-02T15:04.999999999",
	} {
		tm, err = time.Parse(layout, strings.ToUpper(timestr))
		if err != nil {
			continue
		}
		return
	}
	return
}

var (
	ErrInvalidTwtLine = errors.New("error: invalid twt line parsed")
	ErrInvalidFeed    = errors.New("error: erroneous feed detected")
)

type retwtManager struct{}

func (retwtManager) DecodeJSON(b []byte) (types.Twt, error) { return DecodeJSON(b) }
func (retwtManager) ParseLine(line string, twter types.Twter) (twt types.Twt, err error) {
	return ParseLine(line, twter)
}
func (retwtManager) ParseFile(r io.Reader, twter types.Twter) (types.TwtFile, error) {
	return ParseFile(r, twter)
}

func DefaultTwtManager() {
	types.SetTwtManager(&retwtManager{})
}

type retwtFile struct {
	twter types.Twter
	twts  types.Twts
}

var _ types.TwtFile = retwtFile{}

func (r retwtFile) Twter() types.Twter { return r.twter }
func (r retwtFile) Comment() string    { return "" }
func (r retwtFile) Info() types.Info   { return nil }
func (r retwtFile) Twts() types.Twts   { return r.twts }

type reSubject string

func (r reSubject) Tag() types.TwtTag {
	s := string(r)
	return &reTag{s[1 : len(s)-1]}

}
func (r reSubject) Text() string {
	sp := strings.Fields(string(r))
	if len(sp) > 1 {
		return sp[1]
	}
	return ""
}
func (r reSubject) FormatText() string {
	return FormatMentionsAndTagsForSubject(string(r))
}

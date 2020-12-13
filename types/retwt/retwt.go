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
	"sort"
	"strings"
	"time"

	"github.com/apex/log"
	"github.com/jointwt/twtxt/types"
	"golang.org/x/crypto/blake2b"
)

func init() {
	gob.Register(&reTwt{})
}

const (
	TwtHashLength = 7
)

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
	mentions []types.Mention
	tags     []types.Tag

	fmtOpts types.FmtOpts
}

var _ types.Twt = (*reTwt)(nil)
var _ gob.GobEncoder = (*reTwt)(nil)
var _ gob.GobDecoder = (*reTwt)(nil)

func (twt *reTwt) GobEncode() ([]byte, error) {
	enc := struct {
		Twter   types.Twter `json:"twter"`
		Text    string      `json:"text"`
		Created time.Time   `json:"created"`
		Hash    string      `json:"hash"`
	}{twt.twter, twt.text, twt.created, twt.hash}
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

func (twt *reTwt) String() string {
	return fmt.Sprintf("%v\t%v", twt.created.Format(time.RFC3339), twt.text)
}

func NewReTwt(twter types.Twter, text string, created time.Time) *reTwt {
	return &reTwt{twter: twter, text: text, created: created}
}

func DecodeJSON(data []byte) (types.Twt, error) {
	twt := &reTwt{}
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

func ParseFile(r io.Reader, twter types.Twter, ttl time.Duration, N int) (types.Twts, types.Twts, error) {
	scanner := bufio.NewScanner(r)

	var (
		twts types.Twts
		old  types.Twts
	)

	oldTime := time.Now().Add(-ttl)

	nLines, nErrors := 0, 0

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

		if ttl > 0 && twt.Created().Before(oldTime) {
			old = append(old, twt)
		} else {
			twts = append(twts, twt)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}

	if (nLines+nErrors > 0) && nLines == nErrors {
		log.Warnf("erroneous feed dtected (nLines + nErrors > 0 && nLines == nErrors): %d/%d", nLines, nErrors)
		return nil, nil, ErrInvalidFeed
	}

	// Sort by CreatedAt timestamp
	sort.Sort(twts)
	sort.Sort(old)

	// Further limit by Max Cache Items
	if N > 0 && len(twts) > N {
		if N > len(twts) {
			N = len(twts)
		}
		twts = twts[:N]
		old = append(old, twts[N:]...)
	}

	return twts, old, nil
}

func (twt *reTwt) Twter() types.Twter { return twt.twter }
func (twt *reTwt) Text() string       { return twt.text }
func (twt *reTwt) MarkdownText() string {
	return formatMentionsAndTags(twt.fmtOpts, twt.text, types.MarkdownFmt)
}
func (twt *reTwt) SetFmtOpts(opts types.FmtOpts) { twt.fmtOpts = opts }
func (twt *reTwt) Created() time.Time            { return twt.created }
func (twt *reTwt) MarshalJSON() ([]byte, error) {
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
		Subject: twt.Subject(),
	})
}

// Mentions ...
func (twt *reTwt) Mentions() types.MentionList {
	if twt == nil {
		return nil
	}
	if twt.mentions != nil {
		return twt.mentions
	}

	seen := make(map[types.Twter]struct{})
	matches := uriMentionsRe.FindAllStringSubmatch(twt.text, -1)
	for _, match := range matches {
		twter := types.Twter{Nick: match[1], URL: match[2]}
		if _, ok := seen[twter]; !ok {
			twt.mentions = append(twt.mentions, &reMention{twter})
			seen[twter] = struct{}{}
		}
	}

	return twt.mentions
}

// Tags ...
func (twt *reTwt) Tags() types.TagList {
	if twt == nil {
		return nil
	}
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
func (twt *reTwt) Subject() string {
	match := subjectRe.FindStringSubmatch(twt.text)
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
func (twt *reTwt) Hash() string {
	if twt.hash != "" {
		return twt.hash
	}

	payload := twt.Twter().URL + "\n" + twt.Created().Format(time.RFC3339) + "\n" + twt.Text()
	sum := blake2b.Sum256([]byte(payload))

	// Base32 is URL-safe, unlike Base64, and shorter than hex.
	encoding := base32.StdEncoding.WithPadding(base32.NoPadding)
	hash := strings.ToLower(encoding.EncodeToString(sum[:]))
	twt.hash = hash[len(hash)-TwtHashLength:]

	return twt.hash
}

func (twt *reTwt) IsZero() bool {
	return twt.Twter().IsZero() && twt.Created().IsZero() && twt.Text() == ""
}

type reMention struct {
	twter types.Twter
}

var _ types.Mention = (*reMention)(nil)

func (m *reMention) Twter() types.Twter { return m.twter }

type reTag struct {
	tag string
}

var _ types.Tag = (*reTag)(nil)

func (t *reTag) Tag() string {
	if t == nil {
		return ""
	}
	return t.tag
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

func (*retwtManager) DecodeJSON(b []byte) (types.Twt, error) { return DecodeJSON(b) }
func (*retwtManager) ParseLine(line string, twter types.Twter) (twt types.Twt, err error) {
	return ParseLine(line, twter)
}
func (*retwtManager) ParseFile(r io.Reader, twter types.Twter, ttl time.Duration, N int) (types.Twts, types.Twts, error) {
	return ParseFile(r, twter, ttl, N)
}

func DefaultTwtManager() {
	types.SetTwtManager(&retwtManager{})
}

package types

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	TwtHashLength = 7
)

// Twter ...
type Twter struct {
	Nick    string
	URL     string
	Avatar  string
	Tagline string
	Follow  map[string]Twter
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
func (twter Twter) String() string { return fmt.Sprintf("%v\t%v", twter.Nick, twter.URL) }

func (twter Twter) Domain() string {
	if sp := strings.SplitN(twter.Nick, "@", 2); len(sp) == 2 {
		return sp[1]
	}
	if url, err := url.Parse(twter.URL); err == nil {
		return url.Hostname()
	}
	return ""
}
func (twter Twter) DomainNick() string {
	if strings.ContainsRune(twter.Nick, '@') {
		return twter.Nick
	}

	if url, err := url.Parse(twter.URL); err == nil {
		return twter.Nick + "@" + url.Hostname()
	}

	return twter.Nick
}

// Twt ...
type Twt interface {
	IsZero() bool
	Clone() Twt

	Twter() Twter
	Created() time.Time

	Hash() string
	Subject() Subject
	Mentions() MentionList
	Links() LinkList
	Tags() TagList

	ExpandLinks(FmtOpts, FeedLookup)
	// ReplaceLinks(FmtOpts)

	// TODO: remove FormatText and add func to update links for UI
	FormatText(TwtTextFormat, FmtOpts) string

	fmt.Formatter
}

type TwtMention interface {
	Twter() Twter
}

type MentionList []TwtMention

func (ml MentionList) Mentions() []string {
	lis := make([]string, len(ml))
	for i := range ml {
		lis[i] = fmt.Sprint(ml[i])
	}
	return lis
}

type TwtTag interface {
	Text() string
	Target() string
}

type TagList []TwtTag

func (tags *TagList) Tags() []string {
	if tags == nil {
		return nil
	}
	lis := make([]string, len(*tags))
	for i, t := range *tags {
		lis[i] = t.Text()
	}
	return lis
}

type TwtLink interface {
	Text() string
	Target() string
}

type LinkList []TwtLink

func (l LinkList) Links() []string {
	lis := make([]string, len(l))
	for i := range l {
		lis[i] = fmt.Sprint(l[i])
	}
	return lis
}

type Subject interface {
	Text() string
	Tag() TwtTag

	String() string
}

// TwtMap ...
type TwtMap map[string]Twt

// Twts typedef to be able to attach sort methods
type Twts []Twt

func (twts Twts) Len() int {
	return len(twts)
}
func (twts Twts) Less(i, j int) bool {
	return twts[i].Created().After(twts[j].Created())
}
func (twts Twts) Swap(i, j int) {
	twts[i], twts[j] = twts[j], twts[i]
}

// Tags ...
func (twts Twts) Tags() TagList {
	var tags TagList
	for _, twt := range twts {
		tags = append(tags, twt.Tags()...)
	}
	return tags
}

func (twts Twts) TagCount() map[string]int {
	tags := make(map[string]int)
	for _, twt := range twts {
		for _, tag := range twt.Tags() {
			if txt := tag.Text(); txt != "" {
				tags[txt]++
			} else {
				tags[tag.Target()]++
			}
		}
	}
	return tags
}

// Mentions ...
func (twts Twts) Mentions() MentionList {
	var mentions MentionList
	for _, twt := range twts {
		mentions = append(mentions, twt.Mentions()...)
	}
	return mentions
}
func (twts Twts) MentionCount() map[string]int {
	mentions := make(map[string]int)
	for _, twt := range twts {
		for _, m := range twt.Mentions() {
			mentions[m.Twter().Nick]++
		}
	}
	return mentions
}

// Links ...
func (twts Twts) Links() LinkList {
	var links LinkList
	for _, twt := range twts {
		links = append(links, twt.Links()...)
	}
	return links
}
func (twts Twts) LinkCount() map[string]int {
	links := make(map[string]int)
	for _, twt := range twts {
		for _, link := range twt.Links() {
			links[link.Target()]++
		}
	}
	return links
}

// Subjects ...
func (twts Twts) Subjects() []Subject {
	var subjects []Subject
	for _, twt := range twts {
		subjects = append(subjects, twt.Subject())
	}
	return subjects
}

func (twts Twts) SubjectCount() map[string]int {
	subjects := make(map[string]int)
	for _, twt := range twts {
		subjects[twt.Subject().String()]++
	}
	return subjects
}

func (twts Twts) Clone() Twts {
	lis := make([]Twt, len(twts))
	for i := range twts {
		lis[i] = twts[i].Clone()
	}
	return lis
}

func (twts *Twts) UnmarshalJSON(data []byte) error {
	if twts == nil {
		return fmt.Errorf("uninitialized *Twts")
	}
	var lis []json.RawMessage
	err := json.Unmarshal(data, &lis)
	if err != nil {
		return err
	}

	for _, b := range lis {
		twt, err := DecodeJSON(b)
		if err != nil {
			return err
		}
		*twts = append(*twts, twt)
	}
	return nil
}

type FmtOpts interface {
	LocalURL() *url.URL
	IsLocalURL(string) bool
	UserURL(string) string
	ExternalURL(nick, uri string) string
	URLForTag(tag string) string
	URLForUser(user string) string
}

// TwtTextFormat represents the format of which the twt text gets formatted to
type TwtTextFormat int

const (
	// MarkdownFmt to use markdown format
	MarkdownFmt TwtTextFormat = iota
	// HTMLFmt to use HTML format
	HTMLFmt
	// TextFmt to use for og:description
	TextFmt
)

var NilTwt = nilTwt{}

type nilTwt struct{}

var _ Twt = NilTwt
var _ gob.GobEncoder = NilTwt
var _ gob.GobDecoder = NilTwt

func (nilTwt) IsZero() bool { return true }
func (nilTwt) Clone() Twt   { return NilTwt }

func (nilTwt) Twter() Twter       { return Twter{} }
func (nilTwt) Created() time.Time { return time.Now() }
func (nilTwt) Text() string       { return "" }

func (nilTwt) Hash() string          { return "" }
func (nilTwt) Subject() Subject      { return nil }
func (nilTwt) Mentions() MentionList { return nil }
func (nilTwt) Tags() TagList         { return nil }
func (nilTwt) Links() LinkList       { return nil }

func (nilTwt) ExpandLinks(FmtOpts, FeedLookup)          {}
func (nilTwt) Format(state fmt.State, c rune)           {}
func (nilTwt) FormatTwt() string                        { return "" }
func (nilTwt) FormatText(TwtTextFormat, FmtOpts) string { return "" }

func (nilTwt) String() string             { return "" }
func (nilTwt) GobDecode([]byte) error     { return ErrNotImplemented }
func (nilTwt) GobEncode() ([]byte, error) { return nil, ErrNotImplemented }

func init() {
	gob.Register(&nilTwt{})
}

type TwtManager interface {
	DecodeJSON([]byte) (Twt, error)
	ParseLine(string, Twter) (Twt, error)
	ParseFile(io.Reader, Twter) (TwtFile, error)
	MakeTwt(twter Twter, ts time.Time, text string) Twt
}

type nilManager struct{}

var _ TwtManager = nilManager{}

func (nilManager) DecodeJSON([]byte) (Twt, error) { panic("twt managernot configured") }
func (nilManager) ParseLine(line string, twter Twter) (twt Twt, err error) {
	panic("twt managernot configured")
}
func (nilManager) ParseFile(r io.Reader, twter Twter) (TwtFile, error) {
	panic("twt managernot configured")
}
func (nilManager) MakeTwt(twter Twter, ts time.Time, text string) Twt {
	panic("twt managernot configured")
}

var ErrNotImplemented = errors.New("not implemented")

var twtManager TwtManager = &nilManager{}

func DecodeJSON(b []byte) (Twt, error) { return twtManager.DecodeJSON(b) }
func ParseLine(line string, twter Twter) (twt Twt, err error) {
	return twtManager.ParseLine(line, twter)
}
func ParseFile(r io.Reader, twter Twter) (TwtFile, error) {
	return twtManager.ParseFile(r, twter)
}
func MakeTwt(twter Twter, ts time.Time, text string) Twt {
	return twtManager.MakeTwt(twter, ts, text)
}

func SetTwtManager(m TwtManager) {
	twtManager = m
}

type TwtFile interface {
	Twter() Twter
	Info() Info
	Twts() Twts
}

type Info interface {
	Followers() []Twter

	KV
}
type KV interface {
	GetN(string, int) (Value, bool)
	GetAll(string) []Value
	fmt.Stringer
}

type Value interface {
	Key() string
	Value() string
}

// SplitTwts into two groupings. The first with created > ttl or at most N. the second all remaining twts.
func SplitTwts(twts Twts, ttl time.Duration, N int) (Twts, Twts) {
	oldTime := time.Now().Add(-ttl)

	sort.Sort(twts)

	pos := 0
	for ; pos < len(twts) && pos < N; pos++ {
		if twts[pos].Created().Before(oldTime) {
			break
		}
	}

	if pos < 1 {
		return nil, twts
	}

	return twts[:pos], twts[pos:]
}

type FeedLookup interface {
	FeedLookup(string) *Twter
}

type FeedLookupFn func(string) *Twter

func (fn FeedLookupFn) FeedLookup(s string) *Twter { return fn(s) }

func NormalizeUsername(username string) string {
	return strings.TrimSpace(strings.ToLower(username))
}

type PreambleFeed struct {
	r   io.Reader
	buf *bytes.Buffer
	pre *strings.Builder
}

func (p *PreambleFeed) Preamble() string {
	return p.pre.String()
}
func (p *PreambleFeed) Read(b []byte) (n int, err error) {
	i := 0
	if p.buf.Len() > 0 {
		i, err = p.buf.Read(b)
		if err != nil && err != io.EOF {
			return i, err
		}

		if err != nil && err == io.EOF {
			err = nil
		}

		if err != nil {
			return i, err
		}
	}

	ri, err := p.r.Read(b[i:])

	return ri + i, err
}
func ReadPreambleFeed(r io.Reader) (*PreambleFeed, error) {
	p := &PreambleFeed{r: r, buf: &bytes.Buffer{}, pre: &strings.Builder{}}
	b := make([]byte, 4096)

	i, err := r.Read(b[:1])
	if err != nil {
		if err == io.EOF {
			return p, nil
		}

		return p, err
	}
	if i > 0 && b[0] != '#' {
		p.buf.Write(b[:i])
		return p, nil
	}

	eof := false
	for !eof {
		pos := bytes.Index(b[:i], []byte{'\n', '\n'})
		if pos > -1 {
			p.pre.Write(b[:pos])
			p.buf.Write(b[pos:i])
			break
		}

		_, err = p.pre.Write(b[:i])
		if err != nil {
			return nil, err
		}

		i, err = r.Read(b)
		if err != nil {
			if err == io.EOF {
				eof = true
				continue
			}

			return p, err
		}
	}

	return p, nil
}

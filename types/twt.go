package types

import (
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"sort"
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

// Twt ...
type Twt interface {
	Twter() Twter
	Text() string
	FormatText(TwtTextFormat, FmtOpts) string
	Created() time.Time
	IsZero() bool
	Hash() string
	Subject() Subject
	Mentions() MentionList
	Tags() TagList

	fmt.Stringer
}

type Mention interface {
	Twter() Twter
}

type MentionList []Mention

type Tag interface {
	Text() string
	Target() string
}

type TagList []Tag

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

type Subject interface {
	Text() string
	FormatText() string
	Tag() Tag
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
func (twts Twts) TagCount() map[string]int {
	tags := make(map[string]int)
	for _, twt := range twts {
		for _, tag := range twt.Tags() {
			tags[tag.Text()]++
		}
	}
	return tags
}

type FmtOpts interface {
	LocalURL() *url.URL
	IsLocalURL(string) bool
	UserURL(string) string
	ExternalURL(nick, uri string) string
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

func (nilTwt) Twter() Twter                             { return Twter{} }
func (nilTwt) Text() string                             { return "" }
func (nilTwt) FormatText(TwtTextFormat, FmtOpts) string { return "" }
func (nilTwt) Created() time.Time                       { return time.Now() }
func (nilTwt) IsZero() bool                             { return true }
func (nilTwt) Hash() string                             { return "" }
func (nilTwt) Subject() Subject                         { return nil }
func (nilTwt) Mentions() MentionList                    { return nil }
func (nilTwt) Tags() TagList                            { return nil }
func (nilTwt) String() string                           { return "" }
func (nilTwt) GobDecode([]byte) error                   { return ErrNotImplemented }
func (nilTwt) GobEncode() ([]byte, error)               { return nil, ErrNotImplemented }

func init() {
	gob.Register(&nilTwt{})
}

type TwtManager interface {
	DecodeJSON([]byte) (Twt, error)
	ParseLine(string, Twter) (Twt, error)
	ParseFile(io.Reader, Twter) (TwtFile, error)
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

var ErrNotImplemented = errors.New("not implemented")

var twtManager TwtManager = &nilManager{}

func DecodeJSON(b []byte) (Twt, error) { return twtManager.DecodeJSON(b) }
func ParseLine(line string, twter Twter) (twt Twt, err error) {
	return twtManager.ParseLine(line, twter)
}
func ParseFile(r io.Reader, twter Twter) (TwtFile, error) {
	return twtManager.ParseFile(r, twter)
}

func SetTwtManager(m TwtManager) {
	twtManager = m
}

type TwtFile interface {
	Twter() Twter
	Info() KV
	Twts() Twts
}

type KV interface {
	GetN(string, int) (string, bool)
	GetAll(string) []string
}

type MetaValue struct {
	key   string
	value string
}
type Meta map[string][]string

func NewMeta(lis ...MetaValue) *Meta {
	kv := make(Meta, len(lis))
	for _, pair := range lis {
		kv[pair.key] = append(kv[pair.key], pair.value)
	}
	return &kv
}
func (m *Meta) Get(key string) (string, bool) {
	return m.GetN(key, 0)
}
func (m *Meta) GetN(key string, n int) (string, bool) {
	if m == nil || *m == nil {
		return "", false
	}
	if lis, ok := (*m)[key]; ok && len(lis) >= n {
		return lis[n], true
	}
	return "", false
}

// SplitTwts into two groupings. The first with created > ttl or at most N. the second all remaining twts.
func SplitTwts(twts Twts, ttl time.Duration, N int) (Twts, Twts) {
	oldTime := time.Now().Add(-ttl)

	sort.Sort(twts)

	pos := 0
	for ; pos < len(twts) && pos < N; pos++ {
		if twts[pos].Created().Before(oldTime) {
			pos-- // current pos is before oldTime. step back one.
			break
		}
	}

	if pos < 1 {
		return nil, twts
	}

	return twts[:pos], twts[pos:]
}

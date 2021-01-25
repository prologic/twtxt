package lextwt

import (
	"encoding/base32"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/jointwt/twtxt/types"
	"golang.org/x/crypto/blake2b"
)

func init() {
	gob.Register(&Twt{})
}

// Elem AST structs

type Elem interface {
	IsNil() bool     // A typed nil will fail `elem == nil` We need to unbox to test.
	Literal() string // value as read from input.
	Clone() Elem     // clone element.
}
type ElemHTML interface {
	FormatHTML() string

	Elem
}
type ElemMarkdown interface {
	Markdown() string // format to markdown.

	Elem
}

type ElemText interface {
	FormatText() string // format to write to disk.

	Elem
}

type Line interface {
	IsNil() bool     // A typed nil will fail `elem == nil` We need to unbox to test.
	Literal() string // value as read from input.
}

type Comment struct {
	comment string
	key     string
	value   string
}

var _ Line = (*Comment)(nil)

func NewComment(comment string) *Comment {
	return &Comment{comment: comment}
}
func NewCommentValue(comment, key, value string) *Comment {
	return &Comment{comment, key, value}
}
func (n Comment) IsNil() bool     { return n.comment == "" }
func (n Comment) Literal() string { return n.comment + "\n" }
func (n Comment) String() string  { return n.Literal() }
func (n Comment) Key() string     { return n.key }
func (n Comment) Value() string   { return n.value }

type Comments []*Comment

var _ types.KV = Comments{}

func (lis Comments) String() string {
	var b strings.Builder
	for _, line := range lis {
		b.WriteString(line.Literal())
	}
	return b.String()
}

func (lis Comments) GetN(key string, n int) (types.Value, bool) {
	idx := make([]int, 0, len(lis))

	for i := range lis {
		if n == 0 && key == lis[i].key {
			return lis[i], true
		}

		if key == lis[i].key {
			idx = append(idx, i)
		}

		if n == len(idx) && key == lis[i].key {
			return lis[i], true
		}
	}

	if n < 0 && -n < len(idx) {
		return lis[idx[len(idx)+n]], true
	}

	return nil, false
}

func (lis Comments) GetAll(prefix string) []types.Value {
	nlis := make([]types.Value, 0, len(lis))

	for i := range lis {
		if lis[i].key == "" {
			continue
		}

		if strings.HasPrefix(lis[i].key, prefix) {
			nlis = append(nlis, lis[i])
		}
	}

	return nlis
}

func (lis Comments) Followers() []types.Twter {
	flis := lis.GetAll("follow")
	nlis := make([]types.Twter, 0, len(flis))

	for _, o := range flis {
		sp := strings.Fields(o.Value())
		if len(sp) < 2 {
			continue
		}
		nlis = append(nlis, types.Twter{Nick: sp[0], URL: sp[1]})
	}

	return nlis
}

type DateTime struct {
	lit string

	dt time.Time
}

// var _ Elem = (*DateTime)(nil)

func NewDateTime(dt time.Time, lit string) *DateTime {
	if lit == "" {
		lit = dt.Format(time.RFC3339)
	}
	return &DateTime{dt: dt, lit: lit}
}
func (n *DateTime) CloneDateTime() *DateTime {
	if n == nil {
		return nil
	}
	return &DateTime{
		n.lit, n.dt,
	}
}
func (n *DateTime) IsNil() bool { return n == nil }
func (n *DateTime) Literal() string {
	if n == nil {
		return ""
	}
	return n.lit
}
func (n *DateTime) String() string { return n.Literal() }
func (n *DateTime) DateTime() time.Time {
	if n == nil {
		return time.Time{}
	}
	return n.dt
}

type Mention struct {
	lit string

	name   string
	domain string
	target string
	url    *url.URL
	err    error
}

var _ Elem = (*Mention)(nil)
var _ types.TwtMention = (*Mention)(nil)

func NewMention(name, target string) *Mention {
	m := &Mention{name: name, target: target}

	switch {
	case name != "" && target == "":
		m.lit = fmt.Sprintf("@%s", name)

	case name != "" && target != "":
		m.lit = fmt.Sprintf("@<%s %s>", name, target)

	case name == "" && target != "":
		m.lit = fmt.Sprintf("@<%s>", target)
	}

	if sp := strings.SplitN(name, "@", 2); len(sp) == 2 {
		m.name = sp[0]
		m.domain = sp[1]
	}

	if m.domain == "" && m.target != "" {
		if url := m.URL(); url != nil {
			m.domain = url.Hostname()
		}
	}

	return m
}
func (n *Mention) Clone() Elem {
	if n == nil {
		return nil
	}
	return &Mention{
		n.lit, n.name, n.domain, n.target, n.url, n.err,
	}
}
func (n *Mention) IsNil() bool        { return n == nil }
func (n *Mention) Twter() types.Twter { return types.Twter{Nick: n.name, URL: n.target} }
func (n *Mention) Literal() string    { return n.lit }
func (n *Mention) String() string     { return n.FormatText() }
func (n *Mention) Name() string       { return n.name }
func (n *Mention) Domain() string {
	if url := n.URL(); n.domain == "" && url != nil {
		n.domain = url.Hostname()
	}
	return n.domain
}
func (n *Mention) Target() string          { return n.target }
func (n *Mention) SetTarget(target string) { n.target, n.url, n.err = target, nil, nil }
func (n *Mention) URL() *url.URL {
	if n.url == nil && n.err == nil {
		n.url, n.err = url.Parse(n.target)
	}
	return n.url
}
func (n *Mention) Err() error {
	n.URL()
	return n.err
}
func (n *Mention) FormatText() string {
	if n.name == "" && n.target != "" {
		return fmt.Sprintf("@<%s>", n.target)
	}

	nick := n.name

	// if n.domain != "" {
	// 	nick += "@" + n.domain
	// }

	if n.target == "" {
		return fmt.Sprintf("@%s", nick)
	}

	return fmt.Sprintf("@<%s %s>", nick, n.target)
}
func (n *Mention) Markdown() string {
	nick := n.name

	if n.domain != "" {
		nick += "@" + n.domain
	}

	if n.target == "" {
		return fmt.Sprintf("@%s", nick)
	}

	return fmt.Sprintf("[@%s](%s#%s)", nick, n.target, n.name)
}
func (n *Mention) FormatHTML() string {
	if n.target == "" {
		nick := n.name
		if n.domain != "" {
			nick += "@" + n.domain
		}
		return fmt.Sprintf("@%s", nick)
	}

	if n.domain != "" {
		return fmt.Sprintf(`<a href="%s">@%s<em>@%s</em></a>`, n.target, n.name, n.domain)
	}

	return fmt.Sprintf(`<a href="%s">@%s</a>`, n.target, n.name)
}

type Tag struct {
	lit string

	tag    string
	target string
	url    *url.URL
	err    error
}

var _ Elem = (*Tag)(nil)
var _ types.TwtTag = (*Tag)(nil)

func NewTag(tag, target string) *Tag {
	m := &Tag{tag: tag, target: target}
	m.lit = m.FormatText()

	return m
}
func (n *Tag) Clone() Elem {
	return n.CloneTag()
}
func (n *Tag) CloneTag() *Tag {
	if n == nil {
		return nil
	}
	return &Tag{
		n.lit, n.tag, n.target, n.url, n.err,
	}
}
func (n *Tag) IsNil() bool     { return n == nil }
func (n *Tag) Literal() string { return n.lit }
func (n *Tag) String() string  { return n.FormatText() }
func (n *Tag) Text() string    { return n.tag }
func (n *Tag) Target() string  { return n.target }
func (n *Tag) SetTarget(target string) {
	if !n.IsNil() {
		n.target, n.url, n.err = target, nil, nil
	}
}
func (n *Tag) Err() error { return n.err }
func (n *Tag) URL() *url.URL {
	if n.url == nil && n.err == nil {
		n.url, n.err = url.Parse(n.target)
	}
	return n.url
}
func (n *Tag) FormatText() string {
	if n.target == "" {
		return fmt.Sprintf("#%s", n.tag)
	}

	if n.tag == "" {
		return fmt.Sprintf("#<%s>", n.target)
	}

	return fmt.Sprintf("#<%s %s>", n.tag, n.target)
}
func (n *Tag) Markdown() string {
	if n.target == "" {
		return fmt.Sprintf("#%s", n.tag)
	}

	if n.tag == "" {
		url := n.URL()
		return fmt.Sprintf("[%s%s](%s)", url.Hostname(), url.Path, n.target)
	}

	return fmt.Sprintf("[#%s](%s)", n.tag, n.target)
}
func (n *Tag) FormatHTML() string {
	if n.target == "" {
		return fmt.Sprintf("#%s", n.tag)
	}

	return fmt.Sprintf(`<a href="%s">#%s</a>`, n.target, n.tag)
}

type Subject struct {
	subject string
	tag     *Tag
}

var _ Elem = (*Subject)(nil)

func NewSubject(text string) *Subject           { return &Subject{subject: text} }
func NewSubjectTag(tag, target string) *Subject { return &Subject{tag: NewTag(tag, target)} }
func (n *Subject) Clone() Elem {
	if n == nil {
		return nil
	}
	return &Subject{
		n.subject,
		n.tag.CloneTag(),
	}
}
func (n *Subject) IsNil() bool { return n == nil }
func (n *Subject) Literal() string {
	if n.tag != nil {
		return "(" + n.tag.Literal() + ")"
	}

	return "(" + n.subject + ")"
}
func (n *Subject) Text() string {
	if n.tag == nil {
		return n.subject
	}
	return n.tag.Literal()
}
func (n *Subject) Tag() types.TwtTag { return n.tag }
func (n *Subject) FormatText() string {
	if n.tag == nil {
		return fmt.Sprintf("(%s)", n.subject)
	}
	return fmt.Sprintf("(%s)", n.tag.Literal())
}
func (n *Subject) String() string {
	if n.tag == nil {
		return fmt.Sprintf("(%s)", n.subject)
	}
	return fmt.Sprintf("(#%s)", n.tag.Text())
}
func (n *Subject) Markdown() string {
	if n.tag == nil {
		return fmt.Sprintf("(%s)", n.subject)
	}
	return fmt.Sprintf("(%s)", n.tag.Markdown())
}
func (n *Subject) FormatHTML() string {
	if n.tag == nil {
		return fmt.Sprintf("(%s)", n.subject)
	}
	return fmt.Sprintf("(%s)", n.tag.FormatHTML())
}

type Text struct {
	lit string
}

var _ Elem = (*Text)(nil)

func NewText(txt string) *Text { return &Text{txt} }
func (n *Text) Clone() Elem {
	if n == nil {
		return nil
	}
	return &Text{n.lit}
}
func (n *Text) IsNil() bool        { return n == nil }
func (n *Text) Literal() string    { return n.lit }
func (n *Text) String() string     { return n.Literal() }
func (n *Text) FormatText() string { return n.Literal() }
func (n *Text) Markdown() string   { return n.Literal() }

type lineSeparator struct{}

var LineSeparator Elem = &lineSeparator{}

func (n *lineSeparator) Clone() Elem        { return LineSeparator }
func (n *lineSeparator) IsNil() bool        { return false }
func (n *lineSeparator) Literal() string    { return "\u2028" }
func (n *lineSeparator) String() string     { return "\n" }
func (n *lineSeparator) FormatText() string { return "\n" }
func (n *lineSeparator) Markdown() string   { return "\n" }

type Link struct {
	linkType LinkType
	text     string
	target   string
}

var _ Elem = (*Link)(nil)

type LinkType int

const (
	LinkStandard LinkType = iota + 1
	LinkMedia
	LinkPlain
	LinkNaked
)

func NewLink(text, target string, linkType LinkType) *Link { return &Link{linkType, text, target} }
func (n *Link) Clone() Elem {
	if n == nil {
		return nil
	}
	return &Link{
		n.linkType, n.text, n.target,
	}
}
func (n *Link) IsNil() bool { return n == nil }
func (n *Link) Literal() string {
	switch n.linkType {
	case LinkNaked:
		return n.target
	case LinkPlain:
		return fmt.Sprintf("<%s>", n.target)
	case LinkMedia:
		return fmt.Sprintf("![%s](%s)", n.text, n.target)
	default:
		return fmt.Sprintf("[%s](%s)", n.text, n.target)
	}
}
func (n *Link) FormatText() string { return n.Literal() }
func (n *Link) Markdown() string   { return n.Literal() }

func (n *Link) String() string {
	return n.Literal()
}
func (n *Link) IsMedia() bool  { return n.linkType == LinkMedia }
func (n *Link) Text() string   { return n.text }
func (n *Link) Target() string { return n.target }

type Code struct {
	codeType CodeType
	lit      string
}

type CodeType int

const (
	CodeInline CodeType = iota + 1
	CodeBlock
)

var _ Elem = (*Code)(nil)

func NewCode(text string, codeType CodeType) *Code { return &Code{codeType, text} }
func (n *Code) Clone() Elem {
	if n == nil {
		return nil
	}
	return &Code{
		n.codeType, n.lit,
	}
}
func (n *Code) IsNil() bool { return n == nil }
func (n *Code) Literal() string {
	if n.codeType == CodeBlock {
		return fmt.Sprintf("```%s```", n.lit)
	}
	return fmt.Sprintf("`%s`", n.lit)
}
func (n *Code) FormatText() string { return n.Literal() }
func (n *Code) Markdown() string   { return n.String() }

// String replaces line separator with newlines
func (n *Code) String() string {
	return strings.ReplaceAll(n.Literal(), "\u2028", "\n")
}

type Twt struct {
	dt       *DateTime
	msg      []Elem
	mentions []*Mention
	tags     []*Tag
	links    []*Link
	hash     string
	subject  *Subject
	twter    types.Twter
	pos      int
}

var _ Line = (*Twt)(nil)
var _ types.Twt = (*Twt)(nil)

func NewTwt(twter types.Twter, dt *DateTime, elems ...Elem) *Twt {
	twt := &Twt{twter: twter, dt: dt, msg: make([]Elem, 0, len(elems))}

	for _, elem := range elems {
		twt.append(elem)
	}

	return twt
}
func ParseText(text string) ([]Elem, error) {
	r := strings.NewReader(" " + text)
	lexer := NewLexer(r)
	lexer.NextTok() // remove first token we added to avoid parsing as comment.
	parser := NewParser(lexer)

	var lis []Elem
	for elem := parser.ParseElem(); elem != nil; elem = parser.ParseElem() {
		parser.push()
		lis = append(lis, elem)
	}
	var err error

	if e := parser.Errs(); len(e) > 0 {
		err = e
	}

	return lis, err
}
func (twt *Twt) append(elem Elem) {
	if elem == nil || elem.IsNil() {
		return
	}

	twt.msg = append(twt.msg, elem)

	if subject, ok := elem.(*Subject); ok {
		if twt.subject == nil {
			twt.subject = subject
		}
		if subject.tag != nil {
			twt.tags = append(twt.tags, subject.tag)
		}
	}

	if tag, ok := elem.(*Tag); ok {
		twt.tags = append(twt.tags, tag)
	}

	if mention, ok := elem.(*Mention); ok {
		twt.mentions = append(twt.mentions, mention)
	}

	if link, ok := elem.(*Link); ok {
		twt.links = append(twt.links, link)
	}
}
func (twt *Twt) IsNil() bool   { return twt == nil }
func (twt *Twt) FilePos() int  { return twt.pos }
func (twt *Twt) IsZero() bool  { return twt.IsNil() || twt.Literal() == "" || twt.Created().IsZero() }
func (twt *Twt) Elems() []Elem { return twt.msg }
func (twt *Twt) Literal() string {
	var b strings.Builder
	b.WriteString(twt.dt.Literal())
	b.WriteRune('\t')
	b.WriteString(twt.LiteralText())
	b.WriteRune('\n')
	return b.String()
}
func (twt *Twt) LiteralText() string {
	var b strings.Builder
	for _, s := range twt.msg {
		if s == nil || s.IsNil() {
			continue
		}
		b.WriteString(s.Literal())
	}
	return b.String()
}
func (twt Twt) Clone() types.Twt {
	return twt.CloneTwt()
}
func (twt Twt) CloneTwt() *Twt {
	msg := make([]Elem, len(twt.msg))
	for i := range twt.msg {
		msg[i] = twt.msg[i].Clone()
	}
	return NewTwt(twt.twter, twt.dt, msg...)
}
func (twt *Twt) Text() string {
	var b strings.Builder
	for _, s := range twt.msg {
		switch s := s.(type) {
		case ElemText:
			b.WriteString(s.FormatText())
		default:
			b.WriteString(s.Literal())
		}
	}
	return b.String()
}
func (twt *Twt) GobEncode() ([]byte, error) {
	twter := twt.Twter()
	s := fmt.Sprintf(
		"%s\t%s\t%s\t%s\t%s",
		twter.Nick,
		twter.URL,
		twter.Avatar,
		twt.Hash(),
		twt.Literal(),
	)
	return []byte(s), nil
}
func (twt *Twt) GobDecode(data []byte) error {
	sp := strings.SplitN(string(data), "\t", 5)
	if len(sp) != 5 {
		return fmt.Errorf("unable to decode twt: %s ", data)
	}
	twter := types.Twter{Nick: sp[0], URL: sp[1], Avatar: sp[2]}
	twt.hash = sp[3]
	t, err := ParseLine(sp[4], twter)
	if err != nil {
		return err
	}

	if t, ok := t.(*Twt); ok {
		twt.dt = t.dt
		twt.msg = t.msg
		twt.mentions = t.mentions
		twt.tags = t.tags
		twt.links = t.links
		twt.subject = t.subject
		twt.twter = t.twter
	}

	return nil
}
func (twt Twt) MarshalJSON() ([]byte, error) {
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
		MarkdownText: twt.FormatText(types.MarkdownFmt, nil),

		// Dynamic Fields
		Hash:    twt.Hash(),
		Tags:    tags.Tags(),
		Subject: twt.Subject().String(),
	})
}
func DecodeJSON(data []byte) (types.Twt, error) {
	enc := struct {
		Twter   types.Twter `json:"twter"`
		Text    string      `json:"text"`
		Created time.Time   `json:"created"`
		Hash    string      `json:"hash"`
	}{}
	err := json.Unmarshal(data, &enc)
	if err != nil {
		return types.NilTwt, err
	}

	dt := NewDateTime(enc.Created, "")
	elems, err := ParseText(enc.Text)
	if err != nil {
		return types.NilTwt, err
	}

	twt := NewTwt(enc.Twter, dt, elems...)
	if err != nil {
		return types.NilTwt, err
	}

	twt.hash = enc.Hash

	return twt, nil
}
func (twt Twt) FormatTwt() string {
	var b strings.Builder
	b.WriteString(twt.dt.Literal())
	b.WriteRune('\t')
	for _, s := range twt.msg {
		if s == nil || s.IsNil() {
			continue
		}
		switch s := s.(type) {
		case ElemText:
			b.WriteString(s.FormatText())
		default:
			b.WriteString(s.Literal())
		}
	}
	b.WriteRune('\n')
	return b.String()
}
func (twt Twt) FormatText(mode types.TwtTextFormat, opts types.FmtOpts) string {
	twt = *twt.CloneTwt()

	if opts != nil {
		for i := range twt.tags {
			switch mode {
			case types.TextFmt:
				twt.tags[i].target = ""
			}
		}

		for i := range twt.mentions {
			switch mode {
			case types.TextFmt:
				if twt.mentions[i].domain == "" &&
					opts.IsLocalURL(twt.mentions[i].target) &&
					strings.HasSuffix(twt.mentions[i].target, "/twtxt.txt") {
					twt.mentions[i].domain = opts.LocalURL().Hostname()
				}
				twt.mentions[i].target = ""
			case types.MarkdownFmt, types.HTMLFmt:
				if opts.IsLocalURL(twt.mentions[i].target) && strings.HasSuffix(twt.mentions[i].target, "/twtxt.txt") {
					twt.mentions[i].target = opts.UserURL(twt.mentions[i].target)
				} else {
					if twt.mentions[i].domain == "" {
						if u, err := url.Parse(twt.mentions[i].target); err == nil {
							twt.mentions[i].domain = u.Hostname()
						}
					}
					twt.mentions[i].target = opts.ExternalURL(twt.mentions[i].name, twt.mentions[i].target)
				}
			}
		}
	}

	switch mode {
	case types.TextFmt:
		var b strings.Builder
		for _, s := range twt.msg {
			switch s := s.(type) {
			case ElemText:
				b.WriteString(s.FormatText())
			default:
				b.WriteString(s.Literal())
			}
		}
		return b.String()
	case types.MarkdownFmt:
		var b strings.Builder
		for _, s := range twt.msg {
			switch s := s.(type) {
			case ElemMarkdown:
				b.WriteString(s.Markdown())
			case ElemText:
				b.WriteString(s.FormatText())
			default:
				b.WriteString(s.Literal())
			}
		}
		return b.String()
	case types.HTMLFmt:
		var b strings.Builder
		for _, s := range twt.msg {
			switch s := s.(type) {
			case ElemHTML:
				b.WriteString(s.FormatHTML())
			case ElemMarkdown:
				b.WriteString(s.Markdown())
			case ElemText:
				b.WriteString(s.FormatText())
			default:
				b.WriteString(s.Literal())
			}
		}
		return b.String()

	}
	return twt.Literal()
}
func (twt *Twt) ExpandLinks(opts types.FmtOpts, lookup types.FeedLookup) {
	for i, tag := range twt.tags {
		if tag.target == "" {
			tag.target = opts.URLForTag(tag.tag)
		}
		twt.tags[i] = tag
	}

	for i, m := range twt.mentions {
		if m.target == "" && lookup != nil {
			twter := lookup.FeedLookup(m.name)
			m.name = twter.Nick
			if sp := strings.SplitN(twter.Nick, "@", 2); len(sp) == 2 {
				m.name = sp[0]
				m.domain = sp[1]
			}
			m.target = twter.URL
		}

		twt.mentions[i] = m
	}
}
func (twt Twt) String() string     { return strings.ReplaceAll(twt.Literal(), "\u2028", "\n") }
func (twt Twt) Created() time.Time { return twt.dt.DateTime() }
func (twt Twt) Mentions() types.MentionList {
	lis := make([]types.TwtMention, len(twt.mentions))
	for i := range twt.mentions {
		lis[i] = twt.mentions[i]
	}
	return lis
}
func (twt Twt) Tags() types.TagList {
	lis := make([]types.TwtTag, len(twt.tags))
	for i := range twt.tags {
		lis[i] = twt.tags[i]
	}
	return lis
}
func (twt Twt) Links() types.LinkList {
	lis := make([]types.TwtLink, len(twt.links))
	for i := range twt.links {
		lis[i] = twt.links[i]
	}
	return lis
}
func (twt Twt) Twter() types.Twter { return twt.twter }
func (twt Twt) Hash() string {
	if twt.hash != "" {
		return twt.hash
	}

	payload := fmt.Sprintf(
		"%s\n%s\n%s",
		twt.Twter().URL,
		twt.Created().Format(time.RFC3339),
		twt.LiteralText(),
	)
	sum := blake2b.Sum256([]byte(payload))

	// Base32 is URL-safe, unlike Base64, and shorter than hex.
	encoding := base32.StdEncoding.WithPadding(base32.NoPadding)
	hash := strings.ToLower(encoding.EncodeToString(sum[:]))
	twt.hash = hash[len(hash)-types.TwtHashLength:]

	return twt.hash
}
func (twt Twt) Subject() types.Subject {
	if twt.subject == nil {
		twt.subject = NewSubjectTag(twt.Hash(), "")
	}
	return twt.subject
}

// Twts typedef to be able to attach sort methods
type Twts []*Twt

func (twts Twts) Len() int {
	return len(twts)
}
func (twts Twts) Less(i, j int) bool {
	if twts == nil {
		return false
	}

	return twts[i].Created().After(twts[j].Created())
}
func (twts Twts) Swap(i, j int) {
	twts[i], twts[j] = twts[j], twts[i]
}

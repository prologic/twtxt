package lextwt_test

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jointwt/twtxt/types"
	"github.com/jointwt/twtxt/types/lextwt"
	"github.com/jointwt/twtxt/types/retwt"
	"github.com/matryer/is"
)

type Lexer interface {
	NextTok() bool
	GetTok() lextwt.Token
	Rune() rune
	NextRune() bool
}

func TestLexerRunes(t *testing.T) {
	r := strings.NewReader("hello\u2028there. 👋")
	lexer := lextwt.NewLexer(r)
	values := []rune{'h', 'e', 'l', 'l', 'o', '\u2028', 't', 'h', 'e', 'r', 'e', '.', ' ', '👋'}

	testLexerRunes(t, lexer, values)
}

func testLexerRunes(t *testing.T, lexer Lexer, values []rune) {
	t.Helper()

	is := is.New(t)

	for i, r := range values {
		t.Logf("%d of %d - %v %v", i, len(values), string(lexer.Rune()), string(r))
		is.Equal(lexer.Rune(), r) // parsed == value
		if i < len(values)-1 {
			is.True(lexer.NextRune())
		}
	}
	is.True(!lexer.NextRune())
	is.Equal(lexer.Rune(), lextwt.EOF)
}

func TestLexerTokens(t *testing.T) {
	r := strings.NewReader("# comment\n2016-02-03T23:05:00Z	@<example http://example.org/twtxt.txt>\u2028welcome to twtxt!\n2020-11-13T16:13:22+01:00	@<prologic https://twtxt.net/user/prologic/twtxt.txt> (#<pdrsg2q https://twtxt.net/search?tag=pdrsg2q>) Thanks! [link](index.html) ![](img.png)`` ```hi```gopher://example.com \\")
	values := []lextwt.Token{
		{lextwt.TokHASH, []rune("#")},
		{lextwt.TokSPACE, []rune(" ")},
		{lextwt.TokSTRING, []rune("comment")},
		{lextwt.TokNL, []rune("\n")},
		{lextwt.TokNUMBER, []rune("2016")},
		{lextwt.TokHYPHEN, []rune("-")},
		{lextwt.TokNUMBER, []rune("02")},
		{lextwt.TokHYPHEN, []rune("-")},
		{lextwt.TokNUMBER, []rune("03")},
		{lextwt.TokT, []rune("T")},
		{lextwt.TokNUMBER, []rune("23")},
		{lextwt.TokCOLON, []rune(":")},
		{lextwt.TokNUMBER, []rune("05")},
		{lextwt.TokCOLON, []rune(":")},
		{lextwt.TokNUMBER, []rune("00")},
		{lextwt.TokZ, []rune("Z")},
		{lextwt.TokTAB, []rune("\t")},
		{lextwt.TokAT, []rune("@")},
		{lextwt.TokLT, []rune("<")},
		{lextwt.TokSTRING, []rune("example")},
		{lextwt.TokSPACE, []rune(" ")},
		{lextwt.TokSTRING, []rune("http")},
		{lextwt.TokSCHEME, []rune("://")},
		{lextwt.TokSTRING, []rune("example.org/twtxt.txt")},
		{lextwt.TokGT, []rune(">")},
		{lextwt.TokLS, []rune("\u2028")},
		{lextwt.TokSTRING, []rune("welcome")},
		{lextwt.TokSPACE, []rune(" ")},
		{lextwt.TokSTRING, []rune("to")},
		{lextwt.TokSPACE, []rune(" ")},
		{lextwt.TokSTRING, []rune("twtxt")},
		{lextwt.TokBANG, []rune("!")},
		{lextwt.TokNL, []rune("\n")},
		{lextwt.TokNUMBER, []rune("2020")},
		{lextwt.TokHYPHEN, []rune("-")},
		{lextwt.TokNUMBER, []rune("11")},
		{lextwt.TokHYPHEN, []rune("-")},
		{lextwt.TokNUMBER, []rune("13")},
		{lextwt.TokT, []rune("T")},
		{lextwt.TokNUMBER, []rune("16")},
		{lextwt.TokCOLON, []rune(":")},
		{lextwt.TokNUMBER, []rune("13")},
		{lextwt.TokCOLON, []rune(":")},
		{lextwt.TokNUMBER, []rune("22")},
		{lextwt.TokPLUS, []rune("+")},
		{lextwt.TokNUMBER, []rune("01")},
		{lextwt.TokCOLON, []rune(":")},
		{lextwt.TokNUMBER, []rune("00")},
		{lextwt.TokTAB, []rune("\t")},
		{lextwt.TokAT, []rune("@")},
		{lextwt.TokLT, []rune("<")},
		{lextwt.TokSTRING, []rune("prologic")},
		{lextwt.TokSPACE, []rune(" ")},
		{lextwt.TokSTRING, []rune("https")},
		{lextwt.TokSCHEME, []rune("://")},
		{lextwt.TokSTRING, []rune("twtxt.net/user/prologic/twtxt.txt")},
		{lextwt.TokGT, []rune(">")},
		{lextwt.TokSPACE, []rune(" ")},
		{lextwt.TokLPAREN, []rune("(")},
		{lextwt.TokHASH, []rune("#")},
		{lextwt.TokLT, []rune("<")},
		{lextwt.TokSTRING, []rune("pdrsg2q")},
		{lextwt.TokSPACE, []rune(" ")},
		{lextwt.TokSTRING, []rune("https")},
		{lextwt.TokSCHEME, []rune("://")},
		{lextwt.TokSTRING, []rune("twtxt.net/search?tag=pdrsg2q")},
		{lextwt.TokGT, []rune(">")},
		{lextwt.TokRPAREN, []rune(")")},
		{lextwt.TokSPACE, []rune(" ")},
		{lextwt.TokSTRING, []rune("Thanks")},
		{lextwt.TokBANG, []rune("!")},
		{lextwt.TokSPACE, []rune(" ")},
		{lextwt.TokLBRACK, []rune("[")},
		{lextwt.TokSTRING, []rune("link")},
		{lextwt.TokRBRACK, []rune("]")},
		{lextwt.TokLPAREN, []rune("(")},
		{lextwt.TokSTRING, []rune("index.html")},
		{lextwt.TokRPAREN, []rune(")")},
		{lextwt.TokSPACE, []rune(" ")},
		{lextwt.TokBANG, []rune("!")},
		{lextwt.TokLBRACK, []rune("[")},
		{lextwt.TokRBRACK, []rune("]")},
		{lextwt.TokLPAREN, []rune("(")},
		{lextwt.TokSTRING, []rune("img.png")},
		{lextwt.TokRPAREN, []rune(")")},
		{lextwt.TokCODE, []rune("``")},
		{lextwt.TokSPACE, []rune(" ")},
		{lextwt.TokCODE, []rune("```hi```")},
		{lextwt.TokSTRING, []rune("gopher")},
		{lextwt.TokSCHEME, []rune("://")},
		{lextwt.TokSTRING, []rune("example.com")},
		{lextwt.TokSPACE, []rune(" ")},
		{lextwt.TokBSLASH, []rune("\\")},
	}
	lexer := lextwt.NewLexer(r)
	testLexerTokens(t, lexer, values)
}
func TestLexerEdgecases(t *testing.T) {
	r := strings.NewReader("1-T:2Z\tZed-#<>Ted:")
	lexer := lextwt.NewLexer(r)
	testvalues := []lextwt.Token{
		{lextwt.TokNUMBER, []rune("1")},
		{lextwt.TokHYPHEN, []rune("-")},
		{lextwt.TokT, []rune("T")},
		{lextwt.TokCOLON, []rune(":")},
		{lextwt.TokNUMBER, []rune("2")},
		{lextwt.TokZ, []rune("Z")},
		{lextwt.TokTAB, []rune("\t")},
		{lextwt.TokSTRING, []rune("Zed-")},
		{lextwt.TokHASH, []rune("#")},
		{lextwt.TokLT, []rune("<")},
		{lextwt.TokGT, []rune(">")},
		{lextwt.TokSTRING, []rune("Ted")},
		{lextwt.TokSTRING, []rune(":")},
	}
	testLexerTokens(t, lexer, testvalues)
}

func testLexerTokens(t *testing.T, lexer Lexer, values []lextwt.Token) {
	t.Helper()

	is := is.New(t)

	for i, tt := range values {
		_ = i
		t.Logf("%d - %v %v", i, tt.Type, string(tt.Literal))
		lexer.NextTok()
		is.Equal(lexer.GetTok(), tt) // parsed == value
	}
	lexer.NextTok()
	is.Equal(lexer.GetTok(), lextwt.Token{Type: lextwt.TokEOF, Literal: []rune{-1}})
}

func TestLexerBuffer(t *testing.T) {
	r := strings.NewReader(strings.Repeat(" ", 4094) + "🤔")
	lexer := lextwt.NewLexer(r)
	space := lextwt.Token{lextwt.TokSPACE, []rune(strings.Repeat(" ", 4094))}
	think := lextwt.Token{lextwt.TokSTRING, []rune("🤔")}

	is := is.New(t)

	lexer.NextTok()
	is.Equal(lexer.GetTok(), space) // parsed == value

	lexer.NextTok()
	is.Equal(lexer.GetTok(), think) // parsed == value
}

type dateTestCase struct {
	lit  string
	dt   time.Time
	errs []error
}

func TestParseDateTime(t *testing.T) {
	is := is.New(t)

	tests := []dateTestCase{
		{lit: "2016-02-03T23:05:00Z", dt: time.Date(2016, 2, 3, 23, 5, 0, 0, time.UTC)},
		{lit: "2016-02-03T23:05:00-0700", dt: time.Date(2016, 2, 3, 23, 5, 0, 0, time.FixedZone("UTC-0700", -7*3600+0*60))},
		{lit: "2016-02-03T23:05:00.000001234+08:45", dt: time.Date(2016, 2, 3, 23, 5, 0, 1234, time.FixedZone("UTC+0845", 8*3600+45*60))},
		{lit: "2016-02-03T23:05", dt: time.Date(2016, 2, 3, 23, 5, 0, 0, time.UTC)},
		{lit: "2016-02-03", errs: []error{lextwt.ErrParseToken}},
		{lit: "2016", errs: []error{lextwt.ErrParseToken}},
	}
	for i, tt := range tests {
		r := strings.NewReader(tt.lit)
		lexer := lextwt.NewLexer(r)
		parser := lextwt.NewParser(lexer)
		dt := parser.ParseDateTime()
		t.Logf("TestParseDateTime %d - %v", i, tt.lit)

		if tt.errs == nil {
			is.True(dt != nil)
			is.Equal(tt.lit, dt.Literal()) // src value == parsed value
			is.Equal(tt.dt, dt.DateTime()) // src value == parsed value
		} else {
			is.True(dt == nil)
			for i, e := range parser.Errs() {
				is.True(errors.Is(e, tt.errs[i]))
			}
		}
	}
}

type mentionTestCase struct {
	lit  string
	elem *lextwt.Mention
	errs []error
}

func TestParseMention(t *testing.T) {
	is := is.New(t)
	tests := []mentionTestCase{
		{
			lit:  "@<xuu https://sour.is/xuu/twtxt.txt>",
			elem: lextwt.NewMention("xuu", "https://sour.is/xuu/twtxt.txt"),
		},
		{
			lit:  "@<xuu@sour.is https://sour.is/xuu/twtxt.txt>",
			elem: lextwt.NewMention("xuu@sour.is", "https://sour.is/xuu/twtxt.txt"),
		},
		{
			lit:  "@<https://sour.is/xuu/twtxt.txt>",
			elem: lextwt.NewMention("", "https://sour.is/xuu/twtxt.txt"),
		},
		{
			lit:  "@xuu",
			elem: lextwt.NewMention("xuu", ""),
		},
		{
			lit:  "@xuu@sour.is",
			elem: lextwt.NewMention("xuu@sour.is", ""),
		},
	}

	for i, tt := range tests {
		t.Logf("TestParseMention %d - %v", i, tt.lit)

		r := strings.NewReader(tt.lit)
		lexer := lextwt.NewLexer(r)
		parser := lextwt.NewParser(lexer)
		elem := parser.ParseMention()

		is.True(parser.IsEOF())
		if len(tt.errs) == 0 {
			testParseMention(t, tt.elem, elem)
		}
	}
}

func testParseMention(t *testing.T, expect, elem *lextwt.Mention) {
	t.Helper()

	is := is.New(t)

	is.True(elem != nil)
	is.Equal(elem.Literal(), expect.Literal())
	is.Equal(expect.Name(), elem.Name())
	is.Equal(expect.Domain(), elem.Domain())
	is.Equal(expect.Target(), elem.Target())
}

type tagTestCase struct {
	lit  string
	elem *lextwt.Tag
	errs []error
}

func TestParseTag(t *testing.T) {
	is := is.New(t)
	tests := []tagTestCase{
		{
			lit:  "#<asdfasdf https://sour.is/search?tag=asdfasdf>",
			elem: lextwt.NewTag("asdfasdf", "https://sour.is/search?tag=asdfasdf"),
		},

		{
			lit:  "#<https://sour.is/search?tag=asdfasdf>",
			elem: lextwt.NewTag("", "https://sour.is/search?tag=asdfasdf"),
		},

		{
			lit:  "#asdfasdf",
			elem: lextwt.NewTag("asdfasdf", ""),
		},
	}

	for i, tt := range tests {
		t.Logf("TestParseMention %d - %v", i, tt.lit)

		r := strings.NewReader(" " + tt.lit)
		lexer := lextwt.NewLexer(r)
		lexer.NextTok() // remove first token we added to avoid parsing as comment.
		parser := lextwt.NewParser(lexer)
		elem := parser.ParseTag()

		is.True(parser.IsEOF())
		if len(tt.errs) == 0 {
			testParseTag(t, tt.elem, elem)
		}
	}
}

func testParseTag(t *testing.T, expect, elem *lextwt.Tag) {
	t.Helper()
	is := is.New(t)

	is.True(elem != nil)
	is.Equal(expect.Literal(), elem.Literal())
	is.Equal(expect.Text(), elem.Text())
	is.Equal(expect.Target(), elem.Target())

	url, err := url.Parse(expect.Target())
	eURL, eErr := elem.URL()
	is.Equal(err, eErr)
	is.Equal(url, eURL)
}

type subjectTestCase struct {
	lit  string
	elem *lextwt.Subject
	errs []error
}

func TestParseSubject(t *testing.T) {
	is := is.New(t)

	tests := []subjectTestCase{
		{
			lit:  "(#<asdfasdf https://sour.is/search?tag=asdfasdf>)",
			elem: lextwt.NewSubjectTag("asdfasdf", "https://sour.is/search?tag=asdfasdf"),
		},

		{
			lit:  "(#<https://sour.is/search?tag=asdfasdf>)",
			elem: lextwt.NewSubjectTag("", "https://sour.is/search?tag=asdfasdf"),
		},

		{
			lit:  "(#asdfasdf)",
			elem: lextwt.NewSubjectTag("asdfasdf", ""),
		},
		{
			lit:  "(re: something)",
			elem: lextwt.NewSubject("re: something"),
		},
	}

	for i, tt := range tests {
		t.Logf("TestParseMention %d - %v", i, tt.lit)

		r := strings.NewReader(" " + tt.lit)
		lexer := lextwt.NewLexer(r)
		lexer.NextTok() // remove first token we added to avoid parsing as comment.

		parser := lextwt.NewParser(lexer)

		elem := parser.ParseSubject()

		is.True(parser.IsEOF())
		if len(tt.errs) == 0 {
			testParseSubject(t, tt.elem, elem)
		}
	}
}

func testParseSubject(t *testing.T, expect, elem *lextwt.Subject) {
	is := is.New(t)

	is.Equal(elem.Literal(), expect.Literal())
	is.Equal(expect.Text(), elem.Text())
	if tag, ok := expect.Tag().(*lextwt.Tag); ok && tag != nil {
		testParseTag(t, tag, elem.Tag().(*lextwt.Tag))
	}
}

type linkTestCase struct {
	lit  string
	elem *lextwt.Link
	errs []error
}

func TestParseLink(t *testing.T) {
	is := is.New(t)

	tests := []linkTestCase{
		{
			lit:  "[asdfasdf](https://sour.is/search?tag=asdfasdf)",
			elem: lextwt.NewLink("asdfasdf", "https://sour.is/search?tag=asdfasdf", lextwt.LinkStandard),
		},

		{
			lit:  "[asdfasdf hgfhgf](https://sour.is/search?tag=asdfasdf)",
			elem: lextwt.NewLink("asdfasdf hgfhgf", "https://sour.is/search?tag=asdfasdf", lextwt.LinkStandard),
		},

		{
			lit:  "![](https://sour.is/search?tag=asdfasdf)",
			elem: lextwt.NewLink("", "https://sour.is/search?tag=asdfasdf", lextwt.LinkMedia),
		},

		{
			lit:  "<https://sour.is/search?tag=asdfasdf>",
			elem: lextwt.NewLink("", "https://sour.is/search?tag=asdfasdf", lextwt.LinkPlain),
		},

		{
			lit:  "https://sour.is/search?tag=asdfasdf",
			elem: lextwt.NewLink("", "https://sour.is/search?tag=asdfasdf", lextwt.LinkNaked),
		},
	}

	for i, tt := range tests {
		t.Logf("TestParseLink %d - %v", i, tt.lit)

		r := strings.NewReader(" " + tt.lit)
		lexer := lextwt.NewLexer(r)
		lexer.NextTok() // remove first token we added to avoid parsing as comment.
		parser := lextwt.NewParser(lexer)
		elem := parser.ParseLink()

		is.True(parser.IsEOF())
		if len(tt.errs) == 0 {
			testParseLink(t, tt.elem, elem)
		}
	}
}
func testParseLink(t *testing.T, expect, elem *lextwt.Link) {
	t.Helper()
	is := is.New(t)

	is.True(elem != nil)
	is.Equal(expect.Literal(), elem.Literal())
	is.Equal(expect.Text(), elem.Text())
	is.Equal(expect.Target(), elem.Target())
}

type twtTestCase struct {
	lit       string
	text      string
	md        string
	html      string
	twt       types.Twt
	subject   string
	twter     *types.Twter
	skipRetwt bool
}

func TestParseTwt(t *testing.T) {
	is := is.New(t)

	twter := types.Twter{Nick: "example", URL: "http://example.com/example.txt"}

	tests := []twtTestCase{
		{
			lit: "2016-02-03T23:03:00+00:00	@<example http://example.org/twtxt.txt>\u2028welcome to twtxt!\n",
			text: "@example\nwelcome to twtxt!",
			md:   "[@example](http://example.org#example)\nwelcome to twtxt!",
			html: `<a href="http://example.org">@example<em>@example.org</em></a>` + "\nwelcome to twtxt!",
			twt: lextwt.NewTwt(
				twter,
				lextwt.NewDateTime(parseTime("2016-02-03T23:03:00+00:00"), "2016-02-03T23:03:00+00:00"),
				lextwt.NewMention("example", "http://example.org/twtxt.txt"),
				lextwt.LineSeparator,
				lextwt.NewText("welcome to twtxt"),
				lextwt.NewText("!"),
			),
		},

		{
			lit: "2020-12-25T16:55:57Z	I'm busy, but here's an 1+ [Christmas Tree](https://codegolf.stackexchange.com/questions/4244/code-golf-christmas-edition-how-to-print-out-a-christmas-tree-of-height-n)  ``` . 11+1< (Any unused function name|\"\\\"/1+^<#     \"     (row|\"(Fluff|\"\\\"/^<#               11+\"\"*\"**;               1+           \"\\\"/^<#\"<*)           1           (Mess|/\"\\^/\"\\\"+1+1+^<#               11+\"\"*+\"\"*+;               1+           /\"\\^/\"\\\"+1+1+^<#\"<*)           11+\"\"\"**+;     )     1+ \"\\\"/1+^<#) 11+1<(row) ```",
			twt: lextwt.NewTwt(
				twter,
				lextwt.NewDateTime(parseTime("2020-12-25T16:55:57Z"), "2020-12-25T16:55:57Z"),
				lextwt.NewText("I'm busy, but here's an 1+ "),
				lextwt.NewLink("Christmas Tree", "https://codegolf.stackexchange.com/questions/4244/code-golf-christmas-edition-how-to-print-out-a-christmas-tree-of-height-n", lextwt.LinkStandard),
				lextwt.LineSeparator,
				lextwt.LineSeparator,
				lextwt.NewCode(" . 11+1< (Any unused function name|\"\\\"/1+^<#     \"     (row|\"(Fluff|\"\\\"/^<#               11+\"\"*\"**;               1+           \"\\\"/^<#\"<*)           1           (Mess|/\"\\^/\"\\\"+1+1+^<#               11+\"\"*+\"\"*+;               1+           /\"\\^/\"\\\"+1+1+^<#\"<*)           11+\"\"\"**+;     )     1+ \"\\\"/1+^<#) 11+1<(row) ", lextwt.CodeBlock),
			),
		},
		{
			lit: "2020-12-25T16:57:57Z	@<hirad https://twtxt.net/user/hirad/twtxt.txt> (#<hrqg53a https://twtxt.net/search?tag=hrqg53a>) @<prologic https://twtxt.net/user/prologic/twtxt.txt> make this a blog post plz  And I forgot, [Try It Online Again!](https://tio.run/#jVVbb5tIFH7nV5zgB8DGYJxU7br2Q1IpVausFWXbhxUhCMO4RgszdGbIRZv97d4zYAy2Y7fIRnP5znfuh@JFrhgdr9c9WElZiInrFhGPsxcZPZPMkWW@yLgTs9wtmJDuh/ejD@/eexfn3h9uSiXhBSf4Hi4ZH3rDlA6Lik/TemduKbi7SKlL6CNsjnvgDaAjh2u4ba5uK73wTSkGF74STnK1pTaMR94FIm7SmNCYQCrg0ye4@nv41yVcOCMEX1/egOec4@rz/Dt8vr15PNfSvGBcgngR2pKzHGKWZSSWKaMCNncJ@VkSTRM2iARm9da0bPj3P01LyBIYJUVWClMgdgZz3FoTDfBJl0AZcnNZ7zdnGaEm6nMi/uPRgrMZjNtr9RQcnQf9u4h@kAnoMIAG7Y8C3OngL9OMgGSwIECeSVxKkgT6DokSIc@pND2r1U0LNJAVHf2@F9hgcKMF8)",
			subject: "(#hrqg53a)",
			twt: lextwt.NewTwt(
				twter,
				lextwt.NewDateTime(parseTime("2020-12-25T16:57:57Z"), "2020-12-25T16:57:57Z"),
				lextwt.NewMention("hirad", "https://twtxt.net/user/hirad/twtxt.txt"),
				lextwt.NewText(" "),
				lextwt.NewSubjectTag("hrqg53a", "https://twtxt.net/search?tag=hrqg53a"),
				lextwt.NewText(" "),
				lextwt.NewMention("prologic", "https://twtxt.net/user/prologic/twtxt.txt"),
				lextwt.NewText(" make this a blog post plz"),
				lextwt.LineSeparator,
				lextwt.LineSeparator,
				lextwt.NewText("And I forgot, "),
				lextwt.NewLink("Try It Online Again!", "https://tio.run/#jVVbb5tIFH7nV5zgB8DGYJxU7br2Q1IpVausFWXbhxUhCMO4RgszdGbIRZv97d4zYAy2Y7fIRnP5znfuh@JFrhgdr9c9WElZiInrFhGPsxcZPZPMkWW@yLgTs9wtmJDuh/ejD@/eexfn3h9uSiXhBSf4Hi4ZH3rDlA6Lik/TemduKbi7SKlL6CNsjnvgDaAjh2u4ba5uK73wTSkGF74STnK1pTaMR94FIm7SmNCYQCrg0ye4@nv41yVcOCMEX1/egOec4@rz/Dt8vr15PNfSvGBcgngR2pKzHGKWZSSWKaMCNncJ@VkSTRM2iARm9da0bPj3P01LyBIYJUVWClMgdgZz3FoTDfBJl0AZcnNZ7zdnGaEm6nMi/uPRgrMZjNtr9RQcnQf9u4h@kAnoMIAG7Y8C3OngL9OMgGSwIECeSVxKkgT6DokSIc@pND2r1U0LNJAVHf2@F9hgcKMF8", lextwt.LinkStandard),
			),
		},

		{
			lit: "2020-12-04T21:43:43Z	@<prologic https://twtxt.net/user/prologic/twtxt.txt> (#<63dtg5a https://txt.sour.is/search?tag=63dtg5a>) Web Key Directory: a way to self host your public key. instead of using a central system like pgp.mit.net or OpenPGP.org you have your key on a server you own.   it takes an email@address.com hashes the part before the @ and turns it into `[openpgpkey.]address.com/.well-known/openpgpkey[/address.com]/<hash>`",
			subject: "(#63dtg5a)",
			twt: lextwt.NewTwt(
				twter,
				lextwt.NewDateTime(parseTime("2020-12-04T21:43:43Z"), "2020-12-04T21:43:43Z"),
				lextwt.NewMention("prologic", "https://twtxt.net/user/prologic/twtxt.txt"),
				lextwt.NewText(" "),
				lextwt.NewSubjectTag("63dtg5a", "https://txt.sour.is/search?tag=63dtg5a"),
				lextwt.NewText(" Web Key Directory: a way to self host your public key. instead of using a central system like pgp.mit.net or OpenPGP.org you have your key on a server you own. "),
				lextwt.LineSeparator,
				lextwt.LineSeparator,
				lextwt.NewText("it takes an email@address.com hashes the part before the "),
				lextwt.NewText("@ and turns it into "),
				lextwt.NewCode("[openpgpkey.]address.com/.well-known/openpgpkey[/address.com]/<hash>", lextwt.CodeInline),
			),
		},

		{
			lit: "2020-07-20T06:59:52Z	@<hjertnes https://hjertnes.social/twtxt.txt> Is it okay to have two personas :) I have https://twtxt.net/u/prologic and https://prologic.github.io/twtxt.txt 🤔",
			twt: lextwt.NewTwt(
				twter,
				lextwt.NewDateTime(parseTime("2020-07-20T06:59:52Z"), "2020-07-20T06:59:52Z"),
				lextwt.NewMention("hjertnes", "https://hjertnes.social/twtxt.txt"),
				lextwt.NewText(" Is it okay to have two personas :"),
				lextwt.NewText(") I have "),
				lextwt.NewLink("", "https://twtxt.net/u/prologic", lextwt.LinkNaked),
				lextwt.NewText(" and "),
				lextwt.NewLink("", "https://prologic.github.io/twtxt.txt", lextwt.LinkNaked),
				lextwt.NewText(" 🤔"),
			),
		},

		{
			lit: `2021-01-21T23:25:59Z	Alligator  ![](https://twtxt.net/media/L6g5PMqA2JXX7ra5PWiMsM)  > Guy says to his colleague “just don’t fall in!” She replies “yeah good advice!”  🤣  #AustraliaZoo`,
			twt: lextwt.NewTwt(
				twter,
				lextwt.NewDateTime(parseTime("2021-01-21T23:25:59Z"), "2021-01-21T23:25:59Z"),
				lextwt.NewText("Alligator"),
				lextwt.LineSeparator,
				lextwt.LineSeparator,
				lextwt.NewLink("", "https://twtxt.net/media/L6g5PMqA2JXX7ra5PWiMsM", lextwt.LinkMedia),
				lextwt.LineSeparator,
				lextwt.LineSeparator,
				lextwt.NewText("> Guy says to his colleague “just don’t fall in"),
				lextwt.NewText("!” She replies “yeah good advice"),
				lextwt.NewText("!”"),
				lextwt.LineSeparator,
				lextwt.LineSeparator,
				lextwt.NewText("🤣"),
				lextwt.LineSeparator,
				lextwt.LineSeparator,
				lextwt.NewTag("AustraliaZoo", ""),
			),
		},

		{
			lit: `2021-01-24T02:19:54Z	(#ezmdswq) @<lyse https://lyse.isobeef.org/twtxt.txt> (#ezmdswq) Looks good for me!  ![](https://txt.sour.is/media/353DzAXLDCv43GofSMw6SL)`,
			subject: "(#ezmdswq)",
			twt: lextwt.NewTwt(
				twter,
				lextwt.NewDateTime(parseTime("2021-01-24T02:19:54Z"), "2021-01-24T02:19:54Z"),
				lextwt.NewSubjectTag("ezmdswq", ""),
				lextwt.NewText(" "),
				lextwt.NewMention("lyse", "https://lyse.isobeef.org/twtxt.txt"),
				lextwt.NewText(" "),
				lextwt.NewSubjectTag("ezmdswq", ""),
				lextwt.NewText(" Looks good for me"),
				lextwt.NewText("!  "),
				lextwt.NewLink("", "https://txt.sour.is/media/353DzAXLDCv43GofSMw6SL", lextwt.LinkMedia),
			),
		},

		{
			lit: `2021-01-18T20:45:57Z	#9c913a	Web UI for Picoblog: I'm thinking of something similar to [Saisho Edit](/saisho-edit). #picoblog`,
			md: "[#9c913a](http://example.org/search?tag=9c913a)	Web UI for Picoblog: I'm thinking of something similar to [Saisho Edit](/saisho-edit). [#picoblog](http://example.org/search?tag=picoblog)",
			twt: lextwt.NewTwt(
				twter,
				lextwt.NewDateTime(parseTime("2021-01-18T20:45:57Z"), "2021-01-18T20:45:57Z"),
				lextwt.NewTag("9c913a", ""),
				lextwt.NewText("	Web UI for Picoblog: I'm thinking of something similar to "),
				lextwt.NewLink("Saisho Edit", "/saisho-edit", lextwt.LinkStandard),
				lextwt.NewText(". "),
				lextwt.NewTag("picoblog", ""),
			),
		},

		{
			lit: `2021-02-04T12:54:21Z	https://fosstodon.org/@/105673078150704477`,
			md: "https://fosstodon.org/@/105673078150704477",
			twt: lextwt.NewTwt(
				twter,
				lextwt.NewDateTime(parseTime("2021-02-04T12:54:21Z"), "2021-02-04T12:54:21Z"),
				lextwt.NewLink("", "https://fosstodon.org/@/105673078150704477", lextwt.LinkNaked),
			),
		},

		{
			lit: `2021-02-04T12:54:21Z	@stats.`,
			md: "@stats.",
			twt: lextwt.NewTwt(
				twter,
				lextwt.NewDateTime(parseTime("2021-02-04T12:54:21Z"), "2021-02-04T12:54:21Z"),
				lextwt.NewMention("stats", ""),
				lextwt.NewText("."),
			),
		},

		{
			lit: `2021-02-04T12:54:21Z	a twt witn (not a) subject`,
			subject: "(#czirbha)",
			twt: lextwt.NewTwt(
				twter,
				lextwt.NewDateTime(parseTime("2021-02-04T12:54:21Z"), "2021-02-04T12:54:21Z"),
				lextwt.NewText("a twt witn "),
				lextwt.NewSubject("not a"),
				lextwt.NewText(" subject"),
			),
		},

		{
			lit: `2021-02-04T12:54:21Z	@<other http://example.com/other.txt>	example`,
			twter:     &types.Twter{Nick: "other", URL: "http://example.com/other.txt"},
			skipRetwt: true,
			twt: lextwt.NewTwt(
				types.Twter{Nick: "other", URL: "http://example.com/other.txt"},
				lextwt.NewDateTime(parseTime("2021-02-04T12:54:21Z"), "2021-02-04T12:54:21Z"),
				lextwt.NewMention("other", "http://example.com/other.txt"),
				lextwt.NewText("\texample"),
			),
		},

		{
			lit: `2021-02-18T00:44:45Z	(_just kidding!_)`,
			twt: lextwt.NewTwt(
				twter,
				lextwt.NewDateTime(parseTime("2021-02-18T00:44:45Z"), "2021-02-18T00:44:45Z"),
				lextwt.NewSubject("_just kidding!_"),
			),
		},
	}
	fmtOpts := mockFmtOpts{"http://example.org"}
	for i, tt := range tests {
		t.Logf("TestParseTwt %d\n%v", i, tt.twt)

		r := strings.NewReader(tt.lit)
		lexer := lextwt.NewLexer(r)
		parser := lextwt.NewParser(lexer)
		parser.SetTwter(&twter)
		twt := parser.ParseTwt()

		if !tt.skipRetwt {
			rt, err := retwt.ParseLine(strings.TrimRight(tt.lit, "\n"), twter)
			is.NoErr(err)
			is.True(rt != nil)

			if twt != nil && rt != nil {
				is.Equal(twt.Hash(), rt.Hash())
			}
		}

		is.True(twt != nil)
		if twt != nil {
			testParseTwt(t, tt.twt, twt)
		}
		if tt.text != "" {
			is.Equal(twt.FormatText(types.TextFmt, fmtOpts), tt.text)
		}
		if tt.md != "" {
			is.Equal(twt.FormatText(types.MarkdownFmt, fmtOpts), tt.md)
		}
		if tt.html != "" {
			is.Equal(twt.FormatText(types.HTMLFmt, fmtOpts), tt.html)
		}
		if tt.subject != "" {
			is.Equal(fmt.Sprintf("%c", twt.Subject()), tt.subject)
		}
		if tt.twter != nil {
			is.Equal(twt.Twter().Nick, tt.twter.Nick)
			is.Equal(twt.Twter().URL, tt.twter.URL)
		}
	}
}

func testParseTwt(t *testing.T, expect, elem types.Twt) {
	is := is.New(t)

	is.Equal(expect.Twter(), elem.Twter())
	is.Equal(fmt.Sprintf("%+l", expect), fmt.Sprintf("%+l", elem))

	{
		m := elem.Subject()
		n := expect.Subject()
		testParseSubject(t, n.(*lextwt.Subject), m.(*lextwt.Subject))
	}

	{
		m := elem.Mentions()
		n := expect.Mentions()
		for i := range m {
			t.Log(m[i])
		}
		is.Equal(len(n), len(m))
		for i := range m {
			testParseMention(t, m[i].(*lextwt.Mention), n[i].(*lextwt.Mention))
		}
		is.Equal(n, m)
	}

	{
		m := elem.Tags()
		n := expect.Tags()

		is.Equal(len(n), len(m))
		for i := range m {
			testParseTag(t, m[i].(*lextwt.Tag), n[i].(*lextwt.Tag))
		}
	}

	{
		m := elem.Links()
		n := expect.Links()

		is.Equal(len(n), len(m))
		for i := range m {
			testParseLink(t, m[i].(*lextwt.Link), n[i].(*lextwt.Link))
		}
	}

	{
		m := elem.(*lextwt.Twt).Elems()
		n := expect.(*lextwt.Twt).Elems()
		is.Equal(len(m), len(n)) // len(elem) == len(expect)
		for i, e := range m {
			switch elem := e.(type) {
			case *lextwt.Mention:
				expect, ok := n[i].(*lextwt.Mention)
				is.True(ok)
				testParseMention(t, elem, expect)
			case *lextwt.Tag:
				expect, ok := n[i].(*lextwt.Tag)
				is.True(ok)
				testParseTag(t, elem, expect)
			case *lextwt.Link:
				expect, ok := n[i].(*lextwt.Link)
				is.True(ok)
				testParseLink(t, elem, expect)
			case *lextwt.Subject:
				expect, ok := n[i].(*lextwt.Subject)
				is.True(ok)
				testParseSubject(t, elem, expect)

			default:
				is.Equal(e, n[i])
			}
		}
	}
}

type commentTestCase struct {
	lit   string
	key   string
	value string
}

func TestParseComment(t *testing.T) {
	is := is.New(t)

	tests := []commentTestCase{
		{lit: "# comment\n"},
		{lit: "# key = value\n",
			key: "key", value: "value"},
		{lit: "# key with space = value with space\n",
			key: "key with space", value: "value with space"},
		{lit: "# follower = xuu@sour.is https://sour.is/xuu.txt\n",
			key: "follower", value: "xuu@sour.is https://sour.is/xuu.txt"},
	}
	for i, tt := range tests {
		t.Logf("TestComment %d - %v", i, tt.lit)

		r := strings.NewReader(tt.lit)
		lexer := lextwt.NewLexer(r)
		parser := lextwt.NewParser(lexer)

		elem := parser.ParseComment()

		is.True(elem != nil) // not nil
		if elem != nil {
			is.Equal([]byte(tt.lit), []byte(elem.Literal()))
			is.Equal(tt.key, elem.Key())
			is.Equal(tt.value, elem.Value())
		}
	}
}

type textTestCase struct {
	lit   string
	elems []*lextwt.Text
}

func TestParseText(t *testing.T) {
	is := is.New(t)

	tests := []textTestCase{
		{
			lit: "@ ",
			elems: []*lextwt.Text{
				lextwt.NewText("@ "),
			},
		},
	}
	for i, tt := range tests {
		t.Logf("TestText %d - %v", i, tt.lit)

		r := strings.NewReader(tt.lit)
		lexer := lextwt.NewLexer(r)
		parser := lextwt.NewParser(lexer)

		var lis []lextwt.Elem
		for elem := parser.ParseElem(); elem != nil; elem = parser.ParseElem() {
			lis = append(lis, elem)
		}

		is.Equal(len(tt.elems), len(lis))
		for i, expect := range tt.elems {
			t.Logf("'%s' = '%s'", expect, lis[i])
			is.Equal(expect, lis[i])
		}
	}
}

type fileTestCase struct {
	in       io.Reader
	twter    types.Twter
	override *types.Twter
	out      types.TwtFile
	err      error
}

func TestParseFile(t *testing.T) {
	is := is.New(t)

	twter := types.Twter{Nick: "example", URL: "https://example.com/twtxt.txt"}
	override := types.Twter{Nick: "override", URL: "https://example.com/twtxt.txt"}

	tests := []fileTestCase{
		{
			twter:    twter,
			override: &override,
			in: strings.NewReader(`# My Twtxt!
# nick = override
# url = https://example.com/twtxt.txt
# follows = xuu@txt.sour.is https://txt.sour.is/users/xuu.txt

2016-02-03T23:05:00Z	@<example http://example.org/twtxt.txt>` + "\u2028" + `welcome to twtxt!
2020-12-02T01:04:00Z	This is an OpenPGP proof that connects my OpenPGP key to this Twtxt account. See https://key.sour.is/id/me@sour.is for more.  [Verifying my OpenPGP key: openpgp4fpr:20AE2F310A74EA7CEC3AE69F8B3B0604F164E04F]
2020-11-13T16:13:22+01:00	@<prologic https://twtxt.net/user/prologic/twtxt.txt> (#<pdrsg2q https://twtxt.net/search?tag=pdrsg2q>) Thanks!
`),
			out: lextwt.NewTwtFile(
				override,

				lextwt.Comments{
					lextwt.NewComment("# My Twtxt!"),
					lextwt.NewCommentValue("# nick = override", "nick", "override"),
					lextwt.NewCommentValue("# url = https://example.com/twtxt.txt", "url", "https://example.com/twtxt.txt"),
					lextwt.NewCommentValue("# follows = xuu@txt.sour.is https://txt.sour.is/users/xuu.txt", "follows", "xuu@txt.sour.is https://txt.sour.is/users/xuu.txt"),
				},

				[]types.Twt{
					lextwt.NewTwt(
						override,
						lextwt.NewDateTime(parseTime("2016-02-03T23:05:00Z"), "2016-02-03T23:05:00Z"),
						lextwt.NewMention("example", "http://example.org/twtxt.txt"),
						lextwt.LineSeparator,
						lextwt.NewText("welcome to twtxt"),
						lextwt.NewText("!"),
					),

					lextwt.NewTwt(
						override,
						lextwt.NewDateTime(parseTime("2020-12-02T01:04:00Z"), "2020-12-02T01:04:00Z"),
						lextwt.NewText("This is an OpenPGP proof that connects my OpenPGP key to this Twtxt account. See "),
						lextwt.NewLink("", "https://key.sour.is/id/me@sour.is", lextwt.LinkNaked),
						lextwt.NewText(" for more."),
						lextwt.LineSeparator,
						lextwt.LineSeparator,
						lextwt.NewText("[Verifying my OpenPGP key: openpgp4fpr:20AE2F310A74EA7CEC3AE69F8B3B0604F164E04F]"),
					),

					lextwt.NewTwt(
						override,
						lextwt.NewDateTime(parseTime("2020-11-13T16:13:22+01:00"), "2020-11-13T16:13:22+01:00"),
						lextwt.NewMention("prologic", "https://twtxt.net/user/prologic/twtxt.txt"),
						lextwt.NewText(" "),
						lextwt.NewSubjectTag("pdrsg2q", "https://twtxt.net/search?tag=pdrsg2q"),
						lextwt.NewText(" Thanks"),
						lextwt.NewText("!"),
					),
				},
			),
		},
		{
			twter: twter,
			in:    strings.NewReader(`2016-02-03`),
			out: lextwt.NewTwtFile(
				twter,
				nil,
				[]types.Twt{},
			),
			err: types.ErrInvalidFeed,
		},
	}
	for i, tt := range tests {
		t.Logf("ParseFile %d", i)

		f, err := lextwt.ParseFile(tt.in, tt.twter)
		if tt.err != nil {
			is.True(err == tt.err)
			is.True(f == nil)
			continue
		}

		is.True(err == nil)
		is.True(f != nil)

		if tt.override != nil {
			is.Equal(*tt.override, f.Twter())
		}

		{
			lis := f.Info().GetAll("")
			expect := tt.out.Info().GetAll("")
			is.Equal(len(expect), len(lis))

			for i := range expect {
				is.Equal(expect[i].Key(), lis[i].Key())
				is.Equal(expect[i].Value(), lis[i].Value())
			}

			is.Equal(f.Info().String(), tt.out.Info().String())
		}

		t.Log(f.Info().Followers())
		t.Log(tt.out.Info().Followers())

		{
			lis := f.Twts()
			expect := tt.out.Twts()
			is.Equal(len(expect), len(lis))
			for i := range expect {
				testParseTwt(t, expect[i], lis[i])
			}
		}

	}
}

func parseTime(s string) time.Time {
	if dt, err := time.Parse(time.RFC3339, s); err == nil {
		return dt
	}
	return time.Time{}
}

type testExpandLinksCase struct {
	twt    types.Twt
	target *types.Twter
}

func TestExpandLinks(t *testing.T) {
	twter := types.Twter{Nick: "example", URL: "http://example.com/example.txt"}
	conf := mockFmtOpts{
		localURL: "http://example.com",
	}

	tests := []testExpandLinksCase{
		{
			twt: lextwt.NewTwt(
				twter,
				lextwt.NewDateTime(parseTime("2021-01-24T02:19:54Z"), "2021-01-24T02:19:54Z"),
				lextwt.NewMention("@asdf", ""),
			),
			target: &types.Twter{Nick: "asdf", URL: "http://example.com/asdf.txt"},
		},
	}

	is := is.New(t)

	for i, tt := range tests {
		t.Logf("TestExpandLinks %d - %s", i, tt.target)
		lookup := types.FeedLookupFn(func(s string) *types.Twter { return tt.target })
		tt.twt.ExpandLinks(conf, lookup)
		is.Equal(tt.twt.Mentions()[0].Twter().Nick, tt.target.Nick)
		is.Equal(tt.twt.Mentions()[0].Twter().URL, tt.target.URL)
	}
}

type mockFmtOpts struct {
	localURL string
}

func (m mockFmtOpts) LocalURL() *url.URL { u, _ := url.Parse(m.localURL); return u }
func (m mockFmtOpts) IsLocalURL(url string) bool {
	return strings.HasPrefix(url, m.localURL)
}
func (m mockFmtOpts) UserURL(url string) string {
	if strings.HasSuffix(url, "/twtxt.txt") {
		return strings.TrimSuffix(url, "/twtxt.txt")
	}
	return url
}
func (m mockFmtOpts) ExternalURL(nick, uri string) string {
	return fmt.Sprintf(
		"%s/external?uri=%s&nick=%s",
		strings.TrimSuffix(m.localURL, "/"),
		uri, nick,
	)
}
func (m mockFmtOpts) URLForTag(tag string) string {
	return fmt.Sprintf(
		"%s/search?tag=%s",
		strings.TrimSuffix(m.localURL, "/"),
		tag,
	)
}
func (m mockFmtOpts) URLForUser(username string) string {
	return fmt.Sprintf(
		"%s/user/%s/twtxt.txt",
		strings.TrimSuffix(m.localURL, "/"),
		username,
	)
}

// func TestSomethingWeird(t *testing.T) {
// 	is := is.New(t)
// 	twter := types.Twter{Nick: "prologic", URL: "https://twtxt.net/user/prologic/twtxt.txt"}
// 	res, err := http.Get("https://twtxt.net/user/prologic/twtxt.txt")

// 	is.NoErr(err)
// 	defer res.Body.Close()

// 	b, _ := ioutil.ReadAll(res.Body)

// 	retwt, err := retwt.ParseFile(bytes.NewReader(b), twter)
// 	is.NoErr(err)

// 	letwt, err := lextwt.ParseFile(bytes.NewReader(b), twter)
// 	is.NoErr(err)

// 	Rtwts := retwt.Twts()
// 	Ltwts := letwt.Twts()

// 	t.Logf("R TWTS: %d, L Twts: %d", len(Rtwts), len(Ltwts))

// 	for i := range Rtwts {
// 		t.Log(i)
// 		is.Equal(fmt.Sprint(Rtwts[i]), fmt.Sprint(Ltwts[i]))
// 	}

// 	is.True(false)
// }

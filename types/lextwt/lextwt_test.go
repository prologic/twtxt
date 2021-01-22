package lextwt_test

import (
	"errors"
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
		// t.Logf("%d of %d - %v %v", i, len(values), string(lexer.Rune()), string(r))
		is.Equal(lexer.Rune(), r) // parsed == value
		if i < len(values)-1 {
			is.True(lexer.NextRune())
		}
	}
	is.True(!lexer.NextRune())
	is.Equal(lexer.Rune(), lextwt.EOF)
}

func TestLexerTokens(t *testing.T) {
	r := strings.NewReader("# comment\n2016-02-03T23:05:00Z	@<example http://example.org/twtxt.txt>\u2028welcome to twtxt!\n2020-11-13T16:13:22+01:00	@<prologic https://twtxt.net/user/prologic/twtxt.txt> (#<pdrsg2q https://twtxt.net/search?tag=pdrsg2q>) Thanks! [link](index.html) ![](img.png)`` ```hi```gopher://example.com")
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
		{lextwt.TokAMP, []rune("@")},
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
		{lextwt.TokAMP, []rune("@")},
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
	is.Equal(err, elem.Err())
	is.Equal(url, elem.URL())
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
	lit string
	twt types.Twt
}

func TestParseTwt(t *testing.T) {
	is := is.New(t)

	tests := []twtTestCase{
		{
			lit: "2016-02-03T23:05:00Z	@<example http://example.org/twtxt.txt>\u2028welcome to twtxt!\n",
			twt: lextwt.NewTwt(
				types.Twter{},
				lextwt.NewDateTime(parseTime("2016-02-03T23:05:00Z")),
				lextwt.NewMention("example", "http://example.org/twtxt.txt"),
				lextwt.NewText("\u2028welcome to twtxt!"),
			),
		},

		{
			lit: "2020-12-25T16:55:57Z	I'm busy, but here's an 1+ [Christmas Tree](https://codegolf.stackexchange.com/questions/4244/code-golf-christmas-edition-how-to-print-out-a-christmas-tree-of-height-n)  ``` . 11+1< (Any unused function name|\"\\\"/1+^<#     \"     (row|\"(Fluff|\"\\\"/^<#               11+\"\"*\"**;               1+           \"\\\"/^<#\"<*)           1           (Mess|/\"\\^/\"\\\"+1+1+^<#               11+\"\"*+\"\"*+;               1+           /\"\\^/\"\\\"+1+1+^<#\"<*)           11+\"\"\"**+;     )     1+ \"\\\"/1+^<#) 11+1<(row) ```",
			twt: lextwt.NewTwt(
				types.Twter{},
				lextwt.NewDateTime(parseTime("2020-12-25T16:55:57Z")),
				lextwt.NewText("I'm busy, but here's an 1+ "),
				lextwt.NewLink("Christmas Tree", "https://codegolf.stackexchange.com/questions/4244/code-golf-christmas-edition-how-to-print-out-a-christmas-tree-of-height-n", lextwt.LinkStandard),
				lextwt.LineSeparator,
				lextwt.LineSeparator,
				lextwt.NewCode(" . 11+1< (Any unused function name|\"\\\"/1+^<#     \"     (row|\"(Fluff|\"\\\"/^<#               11+\"\"*\"**;               1+           \"\\\"/^<#\"<*)           1           (Mess|/\"\\^/\"\\\"+1+1+^<#               11+\"\"*+\"\"*+;               1+           /\"\\^/\"\\\"+1+1+^<#\"<*)           11+\"\"\"**+;     )     1+ \"\\\"/1+^<#) 11+1<(row) ", lextwt.CodeBlock),
			),
		},
		{
			lit: "2020-12-25T16:57:57Z	@<hirad https://twtxt.net/user/hirad/twtxt.txt> (#<hrqg53a https://twtxt.net/search?tag=hrqg53a>) @<prologic https://twtxt.net/user/prologic/twtxt.txt> make this a blog post plz  And I forgot, [Try It Online Again!](https://tio.run/#jVVbb5tIFH7nV5zgB8DGYJxU7br2Q1IpVausFWXbhxUhCMO4RgszdGbIRZv97d4zYAy2Y7fIRnP5znfuh@JFrhgdr9c9WElZiInrFhGPsxcZPZPMkWW@yLgTs9wtmJDuh/ejD@/eexfn3h9uSiXhBSf4Hi4ZH3rDlA6Lik/TemduKbi7SKlL6CNsjnvgDaAjh2u4ba5uK73wTSkGF74STnK1pTaMR94FIm7SmNCYQCrg0ye4@nv41yVcOCMEX1/egOec4@rz/Dt8vr15PNfSvGBcgngR2pKzHGKWZSSWKaMCNncJ@VkSTRM2iARm9da0bPj3P01LyBIYJUVWClMgdgZz3FoTDfBJl0AZcnNZ7zdnGaEm6nMi/uPRgrMZjNtr9RQcnQf9u4h@kAnoMIAG7Y8C3OngL9OMgGSwIECeSVxKkgT6DokSIc@pND2r1U0LNJAVHf2@F9hgcKMF8)",
			twt: lextwt.NewTwt(
				types.Twter{},
				lextwt.NewDateTime(parseTime("2020-12-25T16:57:57Z")),
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
			twt: lextwt.NewTwt(
				types.Twter{},
				lextwt.NewDateTime(parseTime("2020-12-04T21:43:43Z")),
				lextwt.NewMention("prologic", "https://twtxt.net/user/prologic/twtxt.txt"),
				lextwt.NewText(" "),
				lextwt.NewSubjectTag("63dtg5a", "https://txt.sour.is/search?tag=63dtg5a"),
				lextwt.NewText(" Web Key Directory: a way to self host your public key. instead of using a central system like pgp.mit.net or OpenPGP.org you have your key on a server you own. "),
				lextwt.LineSeparator,
				lextwt.LineSeparator,
				lextwt.NewText("it takes an email@address.com hashes the part before the @ and turns it into "),
				lextwt.NewCode("[openpgpkey.]address.com/.well-known/openpgpkey[/address.com]/<hash>", lextwt.CodeInline),
			),
		},

		{
			lit: "2020-07-20T06:59:52Z	@<hjertnes https://hjertnes.social/twtxt.txt> Is it okay to have two personas :) I have https://twtxt.net/u/prologic and https://prologic.github.io/twtxt.txt 🤔",
			twt: lextwt.NewTwt(
				types.Twter{},
				lextwt.NewDateTime(parseTime("2020-07-20T06:59:52Z")),
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
				types.Twter{},
				lextwt.NewDateTime(parseTime("2021-01-21T23:25:59Z")),
				lextwt.NewText("Alligator"),
				lextwt.LineSeparator,
				lextwt.LineSeparator,
				lextwt.NewLink("", "https://twtxt.net/media/L6g5PMqA2JXX7ra5PWiMsM", lextwt.LinkMedia),
				lextwt.LineSeparator,
				lextwt.LineSeparator,
				lextwt.NewText("> Guy says to his colleague “just don’t fall in!” She replies “yeah good advice!”"),
				lextwt.LineSeparator,
				lextwt.LineSeparator,
				lextwt.NewText("🤣"),
				lextwt.LineSeparator,
				lextwt.LineSeparator,
				lextwt.NewTag("AustraliaZoo", ""),
			),
		},
	}

	for i, tt := range tests {
		t.Logf("TestParseTwt %d\n%v", i, tt.twt.String())

		r := strings.NewReader(tt.lit)
		lexer := lextwt.NewLexer(r)
		parser := lextwt.NewParser(lexer)
		twt := parser.ParseTwt()

		// t.Log(twt.FormatText(types.HTMLFmt, nil))

		rt, err := retwt.ParseLine(strings.TrimRight(tt.lit, "\n"), types.Twter{})
		is.NoErr(err)
		// t.Log(rt.FormatText(types.HTMLFmt, nil))

		is.Equal(twt.FormatText(types.MarkdownFmt, nil), rt.FormatText(types.MarkdownFmt, nil))
		is.Equal(twt.FormatText(types.HTMLFmt, nil), rt.FormatText(types.HTMLFmt, nil))

		is.True(twt != nil)
		if twt != nil {
			testParseTwt(t, tt.twt, twt)
		}
	}
	for i, tt := range tests {
		t.Logf("TestMakeTwt %d\n%v", i, tt.twt.String())
		sp := strings.SplitN(tt.lit, "\t", 2)

		twt := lextwt.MakeTwt(types.Twter{}, parseTime(sp[0]), sp[1])

		is.True(twt != nil)
		if twt != nil {
			testParseTwt(t, tt.twt, twt)
		}
	}
}

func testParseTwt(t *testing.T, expect, elem types.Twt) {
	is := is.New(t)

	is.Equal(expect.Twter(), elem.Twter())
	is.Equal(expect.String(), elem.String())

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
	in    io.Reader
	twter types.Twter
	out   types.TwtFile
}

func TestParseFile(t *testing.T) {
	is := is.New(t)

	twter := types.Twter{Nick: "example", URL: "https://example.com/twtxt.txt"}
	tests := []fileTestCase{
		{
			twter: twter,
			in: strings.NewReader(`# My Twtxt!
# nick = example
# url = https://example.com/twtxt.txt
# follows = xuu@txt.sour.is https://txt.sour.is/users/xuu.txt

2016-02-03T23:05:00Z	@<example http://example.org/twtxt.txt>` + "\u2028" + `welcome to twtxt!
2020-11-13T16:13:22+01:00	@<prologic https://twtxt.net/user/prologic/twtxt.txt> (#<pdrsg2q https://twtxt.net/search?tag=pdrsg2q>) Thanks!
`),
			out: lextwt.NewTwtFile(
				twter,

				lextwt.Comments{
					lextwt.NewComment("# My Twtxt!"),
					lextwt.NewCommentValue("# nick = example", "nick", "example"),
					lextwt.NewCommentValue("# url = https://example.com/twtxt.txt", "url", "https://example.com/twtxt.txt"),
					lextwt.NewCommentValue("# follows = xuu@txt.sour.is https://txt.sour.is/users/xuu.txt", "follows", "xuu@txt.sour.is https://txt.sour.is/users/xuu.txt"),
				},

				[]types.Twt{
					lextwt.NewTwt(
						twter,
						lextwt.NewDateTime(parseTime("2016-02-03T23:05:00Z")),
						lextwt.NewMention("example", "http://example.org/twtxt.txt"),
						lextwt.LineSeparator,
						lextwt.NewText("welcome to twtxt"),
						lextwt.NewText("!"),
					),

					lextwt.NewTwt(
						twter,
						lextwt.NewDateTime(parseTime("2020-11-13T16:13:22+01:00")),
						lextwt.NewMention("prologic", "https://twtxt.net/user/prologic/twtxt.txt"),
						lextwt.NewText(" "),
						lextwt.NewSubjectTag("pdrsg2q", "https://twtxt.net/search?tag=pdrsg2q"),
						lextwt.NewText(" Thanks"),
						lextwt.NewText("!"),
					),
				},
			),
		},
	}
	for i, tt := range tests {
		t.Logf("ParseFile %d", i)

		f, err := lextwt.ParseFile(tt.in, tt.twter)
		is.True(err == nil)
		is.True(f != nil)

		is.Equal(tt.twter, f.Twter())

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
		localURL:   func() *url.URL { url, _ := url.Parse("http://example.com"); return url },
		isLocalURL: func(s string) bool { return strings.HasPrefix("http://example.com", s) },
	}

	tests := []testExpandLinksCase{
		{
			twt:    lextwt.MakeTwt(twter, time.Date(2021, 01, 01, 10, 45, 00, 0, time.UTC), "@asdf"),
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
	localURL    func() *url.URL
	isLocalURL  func(string) bool
	userURL     func(string) string
	externalURL func(string, string) string
	urlForTag   func(string) string
	urlForUser  func(string) string
}

func (m mockFmtOpts) LocalURL() *url.URL                  { return m.localURL() }
func (m mockFmtOpts) IsLocalURL(s string) bool            { return m.isLocalURL(s) }
func (m mockFmtOpts) UserURL(s string) string             { return m.userURL(s) }
func (m mockFmtOpts) ExternalURL(nick, uri string) string { return m.externalURL(nick, uri) }
func (m mockFmtOpts) URLForTag(tag string) string         { return m.urlForTag(tag) }
func (m mockFmtOpts) URLForUser(user string) string       { return m.urlForUser(user) }
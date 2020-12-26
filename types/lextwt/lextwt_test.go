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
	r := strings.NewReader("# comment\n2016-02-03T23:05:00Z	@<example http://example.org/twtxt.txt>\u2028welcome to twtxt!\n2020-11-13T16:13:22+01:00	@<prologic https://twtxt.net/user/prologic/twtxt.txt> (#<pdrsg2q https://twtxt.net/search?tag=pdrsg2q>) Thanks! [link](index.html) ![](img.png)")
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
		{lextwt.TokSTRING, []rune("http://example.org/twtxt.txt")},
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
		{lextwt.TokSTRING, []rune("https://twtxt.net/user/prologic/twtxt.txt")},
		{lextwt.TokGT, []rune(">")},
		{lextwt.TokSPACE, []rune(" ")},
		{lextwt.TokLPAREN, []rune("(")},
		{lextwt.TokHASH, []rune("#")},
		{lextwt.TokLT, []rune("<")},
		{lextwt.TokSTRING, []rune("pdrsg2q")},
		{lextwt.TokSPACE, []rune(" ")},
		{lextwt.TokSTRING, []rune("https://twtxt.net/search?tag=pdrsg2q")},
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
		{lextwt.TokSTRING, []rune("Ted:")},
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
			elem: lextwt.NewSubjectHash(lextwt.NewTag("asdfasdf", "https://sour.is/search?tag=asdfasdf")),
		},

		{
			lit:  "(#<https://sour.is/search?tag=asdfasdf>)",
			elem: lextwt.NewSubjectHash(lextwt.NewTag("", "https://sour.is/search?tag=asdfasdf")),
		},

		{
			lit:  "(#asdfasdf)",
			elem: lextwt.NewSubjectHash(lextwt.NewTag("asdfasdf", "")),
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
			is.Equal(elem.Literal(), tt.lit)
			is.Equal(tt.elem.Text(), elem.Text())
			if tag, ok := tt.elem.Tag().(*lextwt.Tag); ok && tag != nil {
				testParseTag(t, tag, elem.Tag().(*lextwt.Tag))
			}
		}
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
			elem: lextwt.NewLink("asdfasdf", "https://sour.is/search?tag=asdfasdf", false),
		},

		{
			lit:  "![](https://sour.is/search?tag=asdfasdf)",
			elem: lextwt.NewLink("", "https://sour.is/search?tag=asdfasdf", true),
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
	is.Equal(expect.Alt(), elem.Alt())
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
				lextwt.NewDateTime("2016-02-03T23:05:00Z"),
				lextwt.NewMention("example", "http://example.org/twtxt.txt"),
				lextwt.NewText("\u2028welcome to twtxt!"),
			),
		},

		{
			lit: "2020-12-25T16:55:57Z	I'm busy, but here's an 1+ [Christmas Tree](https://codegolf.stackexchange.com/questions/4244/code-golf-christmas-edition-how-to-print-out-a-christmas-tree-of-height-n)  ``` . 11+1< (Any unused function name|\"\\\"/1+^<#     \"     (row|\"(Fluff|\"\\\"/^<#               11+\"\"*\"**;               1+           \"\\\"/^<#\"<*)           1           (Mess|/\"\\^/\"\\\"+1+1+^<#               11+\"\"*+\"\"*+;               1+           /\"\\^/\"\\\"+1+1+^<#\"<*)           11+\"\"\"**+;     )     1+ \"\\\"/1+^<#) 11+1<(row) ```",
			twt: lextwt.NewTwt(
				lextwt.NewDateTime("2020-12-25T16:55:57Z"),
				lextwt.NewText("I'm busy, but here's an 1+ "),
				lextwt.NewLink("Christmas Tree", "https://codegolf.stackexchange.com/questions/4244/code-golf-christmas-edition-how-to-print-out-a-christmas-tree-of-height-n", false),
			),
		},
		// {
		// 	lit: "2020-12-25T16:57:57Z	@<hirad https://twtxt.net/user/hirad/twtxt.txt> (#<hrqg53a https://twtxt.net/search?tag=hrqg53a>) @<prologic https://twtxt.net/user/prologic/twtxt.txt> make this a blog post plz  And I forgot, [Try It Online Again!](https://tio.run/##<jVVbb5tIFH7nV5zgB8DGYJxU7br2Q1IpVausFWXbhxUhCMO4RgszdGbIRZv97d4zYAy2Y7fIRnP5znfuh https://twtxt.net/search?tag=jVVbb5tIFH7nV5zgB8DGYJxU7br2Q1IpVausFWXbhxUhCMO4RgszdGbIRZv97d4zYAy2Y7fIRnP5znfuh>@JFrhgdr9c9WElZiInrFhGPsxcZPZPMkWW@yLgTs9wtmJDuh/ejD@/eexfn3h9uSiXhBSf4Hi4ZH3rDlA6Lik/TemduKbi7SKlL6CNsjnvgDaAjh2u4ba5uK73wTSkGF74STnK1pTaMR94FIm7SmNCYQCrg0ye4@nv41yVcOCMEX1/egOec4@rz/Dt8vr15PNfSvGBcgngR2pKzHGKWZSSWKaMCNncJ@VkSTRM2iARm9da0bPj3P01LyBIYJUVWClMgdgZz3FoTDfBJl0AZcnNZ7zdnGaEm6nMi/uPRgrMZjNtr9RQcnQf9u4h@kAnoMIAG7Y8C3OngL9OMgGSwIECeSVxKkgT6DokSIc@pND2r1U0LNJAVHf2@F9hgcKMF8)"
		// }
	}

	for i, tt := range tests {
		t.Logf("TestParseTwt %d - %v", i, tt.lit)

		r := strings.NewReader(tt.lit)
		lexer := lextwt.NewLexer(r)
		parser := lextwt.NewParser(lexer)
		elem := parser.ParseTwt()

		is.True(elem != nil)
		if elem != nil {
			is.Equal(elem.String(), tt.twt.String())
			is.Equal(len(elem.Mentions()), len(tt.twt.Mentions()))

			m := elem.Mentions()
			n := tt.twt.Mentions()

			for i := range m {
				testParseMention(t, m[i].(*lextwt.Mention), n[i].(*lextwt.Mention))
			}
			is.Equal(elem.Mentions(), tt.twt.Mentions())
		}

	}
}

func TestParseTwts(t *testing.T) {
	r := strings.NewReader("2016-02-03T23:05:00Z	@<example http://example.org/twtxt.txt>\u2028welcome to twtxt!\n2020-11-13T16:13:22+01:00	@<prologic https://twtxt.net/user/prologic/twtxt.txt> (#<pdrsg2q https://twtxt.net/search?tag=pdrsg2q>) Thanks!")
	lexer := lextwt.NewLexer(r)
	parser := lextwt.NewParser(lexer)
	elem := parser.ParseLine()
	t.Logf("%v", elem)
	elem = parser.ParseLine()
	t.Logf("%v", elem)
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

type fileTestCase struct {
	in    io.Reader
	twter types.Twter
	out   types.TwtFile
}

func TestParseFile(t *testing.T) {
	is := is.New(t)

	tests := []fileTestCase{
		{
			twter: types.Twter{Nick: "example", URL: "https://example.com/twtxt.txt"},
			in: strings.NewReader(`
# My Twtxt!
# url = https://example.com/twtxt.txt
# follows = xuu@txt.sour.is https://txt.sour.is/users/xuu.txt

2016-02-03T23:05:00Z	@<example http://example.org/twtxt.txt>\u2028welcome to twtxt!
2020-11-13T16:13:22+01:00	@<prologic https://twtxt.net/user/prologic/twtxt.txt> (#<pdrsg2q https://twtxt.net/search?tag=pdrsg2q>) Thanks!
`),
			out: lextwt.NewTwtFile(
				types.Twter{
					Nick: "example",
					URL:  "https://example.com/twtxt.txt",
				},
				[]types.Twt{
					&lextwt.Twt{},
				},
				lextwt.Comments{
					lextwt.NewComment("# My Twtxt!"),
					lextwt.NewCommentValue("# url = https://example.com/twtxt.txt", "follows", "https://example.com/twtxt.txt"),
				},
			),
		},
	}
	for _, tt := range tests {
		f, err := lextwt.ParseFile(tt.in, tt.twter)
		is.True(err == nil)
		is.True(f != nil)

		is.Equal(tt.twter, f.Twter())

		info := f.Info()
		for _, c := range tt.out.Info().(lextwt.Comments) {
			v, ok := info.GetN(c.Key(), 0)
			is.True(ok)
			is.Equal(c.Value(), v)
		}
	}
}

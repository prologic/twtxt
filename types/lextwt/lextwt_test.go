package lextwt_test

import (
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

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
	r := strings.NewReader("# comment\n2016-02-03T23:05:00Z	@<example http://example.org/twtxt.txt>\u2028welcome to twtxt!\n2020-11-13T16:13:22+01:00	@<prologic https://twtxt.net/user/prologic/twtxt.txt> (#<pdrsg2q https://twtxt.net/search?tag=pdrsg2q>) Thanks!")
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
		{lextwt.TokSTRING, []rune("twtxt!")},
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
		{lextwt.TokSTRING, []rune("(")},
		{lextwt.TokHASH, []rune("#")},
		{lextwt.TokLT, []rune("<")},
		{lextwt.TokSTRING, []rune("pdrsg2q")},
		{lextwt.TokSPACE, []rune(" ")},
		{lextwt.TokSTRING, []rune("https://twtxt.net/search?tag=pdrsg2q")},
		{lextwt.TokGT, []rune(">")},
		{lextwt.TokSTRING, []rune(")")},
		{lextwt.TokSPACE, []rune(" ")},
		{lextwt.TokSTRING, []rune("Thanks!")},
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
	lit    string
	name   string
	domain string
	target string
	errs   []error
}

func TestParseMention(t *testing.T) {
	is := is.New(t)

	tests := []mentionTestCase{
		{
			lit:  "@<xuu https://sour.is/xuu/twtxt.txt>",
			name: "xuu", domain: "sour.is", target: "https://sour.is/xuu/twtxt.txt",
		},
		{
			lit:  "@<https://sour.is/xuu/twtxt.txt>",
			name: "", domain: "sour.is", target: "https://sour.is/xuu/twtxt.txt",
		},
	}

	for i, tt := range tests {
		t.Logf("TestParseMention %d - %v", i, tt.lit)

		r := strings.NewReader(tt.lit)
		lexer := lextwt.NewLexer(r)
		parser := lextwt.NewParser(lexer)

		elem := parser.ParseMention()

		if len(tt.errs) == 0 {
			is.Equal(elem.Literal(), tt.lit)
			is.True(elem != nil)
			is.Equal(tt.name, elem.Name())
			is.Equal(tt.domain, elem.Domain())
			is.Equal(tt.target, elem.Target())
		}
	}
}

type tagTestCase struct {
	lit    string
	tag    string
	target string
	errs   []error
}

func TestParseTag(t *testing.T) {
	is := is.New(t)

	tests := []tagTestCase{
		{
			lit: "#<asdfasdf https://sour.is/search?tag=asdfasdf>",
			tag: "asdfasdf", target: "https://sour.is/search?tag=asdfasdf",
		},

		{
			lit:    "#<https://sour.is/search?tag=asdfasdf>",
			target: "https://sour.is/search?tag=asdfasdf",
		},

		{
			lit: "#asdfasdf",
			tag: "asdfasdf",
		},
	}

	for i, tt := range tests {
		t.Logf("TestParseMention %d - %v", i, tt.lit)

		r := strings.NewReader(" " + tt.lit)
		lexer := lextwt.NewLexer(r)
		lexer.NextTok() // remove first token we added to avoid parsing as comment.

		parser := lextwt.NewParser(lexer)

		elem := parser.ParseTag()

		if len(tt.errs) == 0 {
			is.Equal(elem.Literal(), tt.lit)
			is.True(elem != nil)
			is.Equal(tt.tag, elem.Tag())
			is.Equal(tt.target, elem.Target())

			url, err := url.Parse(tt.target)
			is.Equal(err, elem.Err())
			is.Equal(url, elem.URL())
		}
	}
}

func TestParseTwt(t *testing.T) {
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
		{lit: "# key = value\n", key: "key", value: "value"},
		{lit: "# key with space = value with space\n", key: "key with space", value: "value with space"},
	}
	for i, tt := range tests {
		t.Logf("TestComment %d - %v", i, tt.lit)

		r := strings.NewReader(tt.lit)
		lexer := lextwt.NewLexer(r)
		parser := lextwt.NewParser(lexer)

		elem := parser.ParseComment()

		is.True(elem != nil)
		is.Equal([]byte(tt.lit), []byte(elem.Literal()))
		key, value := elem.KeyValue()
		is.Equal(tt.key, key)
		is.Equal(tt.value, value)
	}
}

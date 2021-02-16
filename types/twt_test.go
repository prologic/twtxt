package types_test

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/jointwt/twtxt/types"
	"github.com/jointwt/twtxt/types/lextwt"
	"github.com/jointwt/twtxt/types/retwt"
	"github.com/matryer/is"
)

// BenchmarkLextwt-16    	      21	  49342715 ns/op	 6567316 B/op	  178333 allocs/op
func BenchmarkAll(b *testing.B) {
	f, err := os.Open("../bench-twtxt.txt")
	if err != nil {
		fmt.Println(err)
		b.FailNow()
	}

	wr := nilWriter{}
	twter := types.Twter{Nick: "prologic", URL: "https://twtxt.net/user/prologic/twtxt.txt"}
	opts := mockFmtOpts{"https://twtxt.net"}

	parsers := []struct {
		name string
		fn   func(r io.Reader, twter types.Twter) (types.TwtFile, error)
	}{
		{"retwt", func(r io.Reader, twter types.Twter) (types.TwtFile, error) { return retwt.ParseFile(r, twter) }},
		{"lextwt", lextwt.ParseFile},
	}

	for _, parser := range parsers {
		b.Run(parser.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = f.Seek(0, 0)
				twts, err := parser.fn(f, twter)
				if err != nil {
					fmt.Println(err)
					b.FailNow()
				}
				for _, twt := range twts.Twts() {
					twt.ExpandLinks(opts, opts)
					fmt.Fprintf(wr, "%h", twt)
				}
			}
		})
	}
}

// BenchmarkLextwtParse-16    	      26	  44508742 ns/op	 5450748 B/op	  130290 allocs/op
func BenchmarkParse(b *testing.B) {
	f, err := os.Open("../bench-twtxt.txt")
	if err != nil {
		fmt.Println(err)
		b.FailNow()
	}

	twter := types.Twter{Nick: "prologic", URL: "https://twtxt.net/user/prologic/twtxt.txt"}

	parsers := []struct {
		name string
		fn   func(r io.Reader, twter types.Twter) (types.TwtFile, error)
	}{
		{"retwt", func(r io.Reader, twter types.Twter) (types.TwtFile, error) { return retwt.ParseFile(r, twter) }},
		{"lextwt", lextwt.ParseFile},
	}

	for _, parser := range parsers {
		b.Run(parser.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = f.Seek(0, 0)
				_, err := parser.fn(f, twter)
				if err != nil {
					fmt.Println(err)
					b.FailNow()
				}
			}
		})
	}
}

func BenchmarkOutput(b *testing.B) {
	f, err := os.Open("../bench-twtxt.txt")
	if err != nil {
		fmt.Println(err)
		b.FailNow()
	}

	wr := nilWriter{}
	twter := types.Twter{Nick: "prologic", URL: "https://twtxt.net/user/prologic/twtxt.txt"}
	opts := mockFmtOpts{"https://twtxt.net"}

	parsers := []struct {
		name string
		fn   func(r io.Reader, twter types.Twter) (types.TwtFile, error)
	}{
		{"retwt", func(r io.Reader, twter types.Twter) (types.TwtFile, error) { return retwt.ParseFile(r, twter) }},
		{"lextwt", lextwt.ParseFile},
	}

	for _, parser := range parsers {
		b.Run(parser.name+"-html", func(b *testing.B) {
			_, _ = f.Seek(0, 0)
			twts, err := parser.fn(f, twter)
			if err != nil {
				fmt.Println(err)
				b.FailNow()
			}

			for i := 0; i < b.N; i++ {
				for _, twt := range twts.Twts() {
					twt.ExpandLinks(opts, opts)
					twt.FormatText(types.HTMLFmt, opts)
				}
			}
		})

		b.Run(parser.name+"-markdown", func(b *testing.B) {
			_, _ = f.Seek(0, 0)
			twts, err := parser.fn(f, twter)
			if err != nil {
				fmt.Println(err)
				b.FailNow()
			}

			for i := 0; i < b.N; i++ {
				for _, twt := range twts.Twts() {
					twt.ExpandLinks(opts, opts)
					twt.FormatText(types.MarkdownFmt, opts)
				}
			}
		})

		b.Run(parser.name+"-text", func(b *testing.B) {
			_, _ = f.Seek(0, 0)
			twts, err := parser.fn(f, twter)
			if err != nil {
				fmt.Println(err)
				b.FailNow()
			}

			for i := 0; i < b.N; i++ {
				for _, twt := range twts.Twts() {
					twt.ExpandLinks(opts, opts)
					twt.FormatText(types.TextFmt, opts)
				}
			}
		})

		b.Run(parser.name+"-literal", func(b *testing.B) {
			_, _ = f.Seek(0, 0)
			twts, err := parser.fn(f, twter)
			if err != nil {
				fmt.Println(err)
				b.FailNow()
			}

			for i := 0; i < b.N; i++ {
				for _, twt := range twts.Twts() {
					twt.ExpandLinks(opts, opts)
					fmt.Fprintf(wr, "%+l", twt)
				}
			}
		})

	}
}

type nilWriter struct{}

func (nilWriter) Write([]byte) (int, error) { return 0, nil }

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
func (m mockFmtOpts) FeedLookup(s string) *types.Twter {
	return &types.Twter{Nick: s, URL: fmt.Sprintf("https://example.com/users/%s/twtxt.txt", s)}
}

type preambleTestCase struct {
	in string
	preamble string
	drain string
}

func TestPreambleFeed(t *testing.T) {
	tests := []preambleTestCase{
		{
			in: "# testing\n\n2020-...",
			preamble: "# testing",
			drain: "\n\n2020-...",
		},

		{
			in: "# testing\nmulti\nlines\n\n2020-...",
			preamble: "# testing\nmulti\nlines",
			drain: "\n\n2020-...",
		},

		{
			in: "2020-...NO PREAMBLE",
			preamble: "",
			drain: "2020-...NO PREAMBLE",
		},

		{
			in: "#onlyonen\n2020-...OOPS ALL PREAMBLE",
			preamble: "#onlyonen\n2020-...OOPS ALL PREAMBLE",
			drain: "",
		},


		{
			in: "#onlypreamble\n",
			preamble: "#onlypreamble\n",
			drain: "",
		},

		{
			in: "",
			preamble: "",
			drain: "",
		},

		{
			in: "X",
			preamble: "",
			drain: "X",
		},

		{
			in: "#",
			preamble: "#",
			drain: "",
		},
	}

	is := is.New(t)

	for _, tt := range tests {
		pf, err := types.ReadPreambleFeed(strings.NewReader(tt.in))

		is.NoErr(err)
		is.Equal(pf.Preamble(), tt.preamble)
		drain, err := ioutil.ReadAll(pf)
		is.NoErr(err)
		is.Equal(tt.drain, string(drain))
	}
}

package internal

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectFollowerFromUserAgent(t *testing.T) {
	testCases := []struct {
		ua       string
		err      error
		expected *TwtxtUserAgent
	}{
		{
			ua:       `Linguee Bot (http://www.linguee.com/bot; bot@linguee.com)`,
			err:      ErrInvalidUserAgent,
			expected: nil,
		},
		{
			ua:  `twtxt/1.2.3 (+https://foo.com/twtxt.txt; @foo)`,
			err: nil,
			expected: &TwtxtUserAgent{
				Client: "twtxt/1.2.3",
				Nick:   "foo",
				URL:    "https://foo.com/twtxt.txt",
			},
		},
	}

	for _, testCase := range testCases {
		actual, err := DetectFollowerFromUserAgent(testCase.ua)
		if err != nil {
			assert.Equal(t, testCase.err, err)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, testCase.expected.Client, actual.Client)
			assert.Equal(t, testCase.expected.Nick, actual.Nick)
			assert.Equal(t, testCase.expected.URL, actual.URL)
		}
	}
}

func TestFormatMentionsAndTags(t *testing.T) {
	conf := &Config{BaseURL: "http://0.0.0.0:8000"}

	testCases := []struct {
		text     string
		format   TwtTextFormat
		expected string
	}{
		{
			text:     "@<test http://0.0.0.0:8000/user/test/twtxt.txt>",
			format:   HTMLFmt,
			expected: `<a href="http://0.0.0.0:8000/user/test">@test</a>`,
		},
		{
			text:     "@<test http://0.0.0.0:8000/user/test/twtxt.txt>",
			format:   MarkdownFmt,
			expected: "[@test](http://0.0.0.0:8000/user/test/twtxt.txt#test)",
		},
		{
			text:     "@<iamexternal http://iamexternal.com/twtxt.txt>",
			format:   HTMLFmt,
			expected: fmt.Sprintf(`<a href="%s">@iamexternal</a>`, URLForExternalProfile(conf, "iamexternal", "http://iamexternal.com/twtxt.txt")),
		},
		{
			text:     "@<iamexternal http://iamexternal.com/twtxt.txt>",
			format:   MarkdownFmt,
			expected: "[@iamexternal](http://iamexternal.com/twtxt.txt#iamexternal)",
		},
		{
			text:     "#<test http://0.0.0.0:8000/search?tag=test>",
			format:   HTMLFmt,
			expected: `<a href="http://0.0.0.0:8000/search?tag=test">#test</a>`,
		},
		{
			text:     "#<test http://0.0.0.0:8000/search?tag=test>",
			format:   MarkdownFmt,
			expected: `[#test](http://0.0.0.0:8000/search?tag=test)`,
		},
	}

	for _, testCase := range testCases {
		actual := FormatMentionsAndTags(conf, testCase.text, testCase.format)
		assert.Equal(t, testCase.expected, actual)
	}
}

package internal

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
			expected: `[@test](http://0.0.0.0:8000/user/test)`,
		},
		{
			text:     "@<iamexternal http://iamexternal.com/twtxt.txt>",
			format:   HTMLFmt,
			expected: fmt.Sprintf(`<a href="%s">@iamexternal</a>`, URLForExternalProfile(conf, "iamexternal", "http://iamexternal.com/twtxt.txt")),
		},
		{
			text:     "@<iamexternal http://iamexternal.com/twtxt.txt>",
			format:   MarkdownFmt,
			expected: fmt.Sprintf(`[@iamexternal](%s)`, URLForExternalProfile(conf, "iamexternal", "http://iamexternal.com/twtxt.txt")),
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

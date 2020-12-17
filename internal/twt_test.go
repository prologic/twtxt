package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExpandTag(t *testing.T) {
	assert := assert.New(t)
	conf := &Config{BaseURL: "http://0.0.0.0:8000"}

	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "does nothing with an empty text",
			input:    "",
			expected: "",
		}, {
			name:     "does nothing with a text without any tag",
			input:    "just regular text",
			expected: "just regular text",
		}, {
			name:     "expands a folded tag",
			input:    "#foo",
			expected: "#<foo http://0.0.0.0:8000/search?tag=foo>",
		}, {
			name:     "expands a folded tag surrounded with spaces",
			input:    "foo #bar baz",
			expected: "foo #<bar http://0.0.0.0:8000/search?tag=bar> baz",
		}, {
			name:     "expands a folded tag in enclosed in parentheses",
			input:    "foo (#bar) baz",
			expected: "foo (#<bar http://0.0.0.0:8000/search?tag=bar>) baz",
		}, {
			name:     "does nothing with an already expanded tag pointing to local instance",
			input:    "#<foo http://0.0.0.0:8000/search?tag=foo>",
			expected: "#<foo http://0.0.0.0:8000/search?tag=foo>",
		}, {
			name:     "does nothing with an already expanded tag pointing somewhere else",
			input:    "#<foo https://example.com/foo>",
			expected: "#<foo https://example.com/foo>",
		}, {
			name:     "does nothing with a plain URL containing an anchor",
			input:    "https://example.com/foo#bar",
			expected: "https://example.com/foo#bar",
		}, {
			name:     "does nothing with a markdown link URL containing an anchor",
			input:    "[foo](https://example.com/foo#bar)",
			expected: "[foo](https://example.com/foo#bar)",
		}, {
			name:     "does nothing with a markdown link title containing an anchor",
			input:    "[#bar](https://example.com/foo)",
			expected: "[#bar](https://example.com/foo)",
		}, {
			name:     "does nothing with a markdown link title/URL containing an anchor",
			input:    "[#bar](https://example.com/foo#bar)",
			expected: "[#bar](https://example.com/foo#bar)",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(testCase.expected, ExpandTag(conf, testCase.input))
		})
	}
}

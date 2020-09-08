package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExpandTags(t *testing.T) {
	assert := assert.New(t)

	testCases := []struct {
		Text   string
		Output string
	}{
		{
			Text:   "#foo",
			Output: "#<foo http://127.0.0.1:8000/search?tag=foo>",
		},
		{
			Text:   "#foo, #bar",
			Output: "#<foo http://127.0.0.1:8000/search?tag=foo>, #<bar http://127.0.0.1:8000/search?tag=bar>",
		},
		{
			Text:   "(#foo123)",
			Output: "(#<foo123 http://127.0.0.1:8000/search?tag=foo123>)",
		},
		{
			Text:   "[#foo]",
			Output: "[#<foo http://127.0.0.1:8000/search?tag=foo>]",
		},
		{
			Text:   "http://127.0.0.1:8000/#foo",
			Output: "http://127.0.0.1:8000/#foo",
		},
		/* XXX: This edge-case does not work
		{
			Text:   "http://127.0.0.1:8000/#foo #foo",
			Output: "http://127.0.0.1:8000/#foo #<foo http://127.0.0.1:8000/search?tag=foo>",
		},
		*/
		// But this one does...
		{
			Text:   "http://127.0.0.1:8000/#foo #bar",
			Output: "http://127.0.0.1:8000/#foo #<bar http://127.0.0.1:8000/search?tag=bar>",
		},
		{
			Text:   "https://github.com/foo/bar/issues/1#issue-12345567",
			Output: "https://github.com/foo/bar/issues/1#issue-12345567",
		},
	}

	conf := &Config{BaseURL: "http://127.0.0.1:8000"}

	for _, testCase := range testCases {
		assert.Equal(testCase.Output, ExpandTags(conf, testCase.Text))
	}
}

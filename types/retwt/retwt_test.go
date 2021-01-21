package retwt_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/jointwt/twtxt/types"
	"github.com/jointwt/twtxt/types/retwt"
	"github.com/stretchr/testify/assert"
)

type TestCase struct {
	Name     string
	Input    string
	Expected string
}

func (tc TestCase) String() string {
	return tc.Name
}

func TestSubject(t *testing.T) {
	assert := assert.New(t)

	testCases := []TestCase{
		{
			Name:     "Single mention with subject hash",
			Input:    "@<antonio bla.com> (#iuf98kd) nice post!",
			Expected: "(#iuf98kd)",
		}, {
			Name:     "single mention with non-hash subject",
			Input:    "@<prologic bla.com> (re nice jacket)",
			Expected: "(re nice jacket)",
		}, {
			Name:     "no mentions with non-hash subject and no content",
			Input:    "(re nice jacket)",
			Expected: "(re nice jacket)",
		}, {
			Name:     "no mentions, no subject with content and sub-content",
			Input:    "Best time of the week (aka weekend)",
			Expected: "",
		}, {
			Name:     "single mention with non-hash subject, content and sub-content",
			Input:    "@<antonio bla.com> (re weekend) I like the weekend too. (is the best)",
			Expected: "(re weekend)",
		}, {
			Name:     "no mentions, no subject with content and multiple sub-content",
			Input:    "tomorrow (sat) (sun) (moon)",
			Expected: "",
		}, {
			Name:     "multiple mentions with hashed subject and content and multiple sub-content",
			Input:    "@<antonio2 bla.com> @<antonio bla.com> (#j3hyzva) testte #test1 (s) and #test2 (s) and more text",
			Expected: "(#j3hyzva)",
		}, {
			Name:     "multiple mentions, with hashed subject and content",
			Input:    "@<antonio3 bla.com> @<antonio bla.com> (#j3hyzva) testing again",
			Expected: "(#j3hyzva)",
		}, {
			Name:     "no mentions with hashed subject and content",
			Input:    "(#veryfunny) you are funny",
			Expected: "(#veryfunny)",
		}, {
			Name:     "no mentinos, on subject with content and sub-content",
			Input:    "#having fun (saturday) another day",
			Expected: "",
		}, {
			Name:     "single mention with content and no subject",
			Input:    "@<antonio3 bla.com> not funny dude",
			Expected: "",
		}, {
			Name:     "single mention with hashed subject uri and content",
			Input:    "@<prologic foo.com> (#<il5rdfq blah.com>) foo bar baz",
			Expected: "(#il5rdfq)",
		}, {
			Name:     "mixxed mentions with hashed subject uri and content",
			Input:    "@<prologic foo.com> @antonio (#<il5rdfq blah.com>) foo bar baz",
			Expected: "(#il5rdfq)",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.String(), func(t *testing.T) {
			twt := retwt.NewReTwt(types.Twter{}, testCase.Input, time.Now())
			if testCase.Expected == "" {
				assert.Equal(fmt.Sprintf("(#%s)", twt.Hash()), twt.Subject().FormatText())
			} else {
				assert.Equal(testCase.Expected, twt.Subject().FormatText())
			}
		})
	}
}

func TestHash(t *testing.T) {
	assert := assert.New(t)

	CET := time.FixedZone("UTC+1", 1*60*60)
	testCases := []struct {
		name     string
		created  time.Time
		expected string
	}{
		{
			name:     "timestamp with milliseconds precision is truncated to seconds precision",
			created:  time.Date(2020, 12, 9, 16, 38, 42, 123_000_000, CET),
			expected: "64u2m5a",
		}, {
			name:     "timestamp with milliseconds precision is truncated to seconds precision without rounding",
			created:  time.Date(2020, 12, 9, 16, 38, 42, 999_000_000, CET),
			expected: "64u2m5a",
		}, {
			name:     "timestamp with seconds precision and UTC+1 offset is kept intact",
			created:  time.Date(2020, 12, 9, 16, 38, 42, 0, CET),
			expected: "64u2m5a",
		}, {
			name:     "timestamp with minutes precision is expanded to seconds precision",
			created:  time.Date(2020, 12, 9, 16, 38, 0, 0, CET),
			expected: "a3c3k5q",
		}, {
			name:     "timestamp with UTC is rendered as designated Zulu offset rather than numeric offset",
			created:  time.Date(2020, 12, 9, 15, 38, 42, 0, time.UTC),
			expected: "74qtyjq",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			twt := retwt.NewReTwt(
				types.Twter{URL: "https://example.com/twtxt.txt"},
				"The twt hash now uses the RFC 3339 timestamp format.",
				testCase.created,
			)
			assert.Equal(testCase.expected, twt.Hash())
		})
	}
}

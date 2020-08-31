package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSubject(t *testing.T) {
	assert := assert.New(t)

	t.Run("String", func(t *testing.T) {
		testCases := []struct {
			Text    string
			Subject string
		}{
			{
				Text:    "@<antonio bla.com> (#iuf98kd) nice post!",
				Subject: "(#iuf98kd)",
			}, {
				Text:    "@<prologic bla.com> (re nice jacket)",
				Subject: "(re nice jacket)",
			}, {
				Text:    "(re nice jacket)",
				Subject: "(re nice jacket)",
			}, {
				Text:    "Best time of the week (aka weekend)",
				Subject: "",
			}, {
				Text:    "@<antonio bla.com> (re weekend) I like the weekend too. (is the best)",
				Subject: "(re weekend)",
			}, {
				Text:    "tomorrow (sat) (sun) (moon)",
				Subject: "",
			}, {
				Text:    "@<antonio2 bla.com> @<antonio bla.com> (#j3hyzva) testte #test1 (s) and #test2 (s) and more text",
				Subject: "(#j3hyzva)",
			}, {
				Text:    "@<antonio3 bla.com> @<antonio bla.com> (#j3hyzva) testing again",
				Subject: "(#j3hyzva)",
			}, {
				Text:    "(#veryfunny) you are funny",
				Subject: "(#veryfunny)",
			}, {
				Text:    "#having fun (satruday) another day",
				Subject: "",
			}, {
				Text:    "@<antonio3 bla.com> not funny dude",
				Subject: "",
			},
		}

		for _, testCase := range testCases {
			twt := Twt{Twter: Twter{}, Text: testCase.Text, Created: time.Now()}
			assert.Equal(testCase.Subject, twt.Subject())
		}
	})
}

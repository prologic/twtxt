package types

import (
	"testing"
  "time"

	"github.com/stretchr/testify/assert"
)

func TestSubject(t *testing.T) {
	assert := assert.New(t)

	t.Run("String", func(t *testing.T) {
		//f := Feed{Nick: "prologic", URL: "https://twtxt.net/user/prologic/twtxt.txt"}
		//assert.Equal("@<prologic https://twtxt.net/user/prologic/twtxt.txt>", f.String())
    // a := [5][2]string{{"@antonio (#iuf98kd) nice post!","(#iuf98kd)"}, {"",""}, {"",""}, {"",""},{"",""}}

    cases := [][]string{
		              []string{"@<antonio bla.com> (#iuf98kd) nice post!", "(#iuf98kd)"},
		              []string{"@<prologic bla.com> (re nice jacket)", "(re nice jacket)"},
		              []string{"(re nice jacket)", ""},
                  []string{"Best time of the week (aka weekend)", ""},
                  []string{"@<antonio bla.com> (re weekend) I like the weekend too. (is the best)", "(re weekend)"},
                  []string{"tomorrow (sat) (sun) (moon)",""},
                  []string{"@<antonio2 bla.com> @<antonio bla.com> (#j3hyzva) testte #test1 (s) and #test2 (s) and more text","(#j3hyzva)"},
                  []string{"@<antonio3 bla.com> @<antonio bla.com> (#j3hyzva) testing again", "(#j3hyzva)"},
                  []string{"(#veryfunny) you are funny",""},
                  []string{"#having fun (satruday) another day",""},
                  []string{"@<antonio3 bla.com> not funny dude",""},
	               }

    for i := 0; i < len(cases); i++ {
      twt := Twt{Twter: Twter{}, Text: cases[i][0], Created: time.Now()}
      sub := twt.Subject()
      assert.Equal(cases[i][1], sub)
    }
	})
}

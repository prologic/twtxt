package lextwt

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/apex/log"
	"github.com/jointwt/twtxt/types"
)

func DefaultTwtManager() {
	types.SetTwtManager(&lextwtManager{})
}

// ParseFile and return time & count limited twts + comments
func ParseFile(r io.Reader, twter types.Twter) (types.TwtFile, error) {

	f := &lextwtFile{twter: twter}

	nLines, nErrors := 0, 0

	lexer := NewLexer(r)
	parser := NewParser(lexer)
	parser.SetTwter(twter)

	for !parser.IsEOF() {
		line := parser.ParseLine()

		nLines++

		switch e := line.(type) {
		case *Comment:
			f.comments = append(f.comments, e)
		case *Twt:
			f.twts = append(f.twts, e)
		}
	}
	nErrors = len(parser.Errs())

	if (nLines+nErrors > 0) && nLines == nErrors {
		log.Warnf("erroneous feed dtected (nLines + nErrors > 0 && nLines == nErrors): %d/%d", nLines, nErrors)
		// return nil, ErrParseElm
	}

	if v, ok := f.Info().GetN("nick", 0); ok {
		f.twter.Nick = v.Value()
	}

	if v, ok := f.Info().GetN("url", 0); ok {
		f.twter.URL = v.Value()
	}

	if v, ok := f.Info().GetN("twturl", 0); ok {
		f.twter.URL = v.Value()
	}

	return f, nil
}
func ParseLine(line string, twter types.Twter) (twt types.Twt, err error) {
	if line == "" {
		return types.NilTwt, nil
	}

	r := strings.NewReader(line)
	lexer := NewLexer(r)
	parser := NewParser(lexer)
	parser.SetTwter(twter)

	twt = parser.ParseTwt()

	if twt.IsZero() {
		return types.NilTwt, fmt.Errorf("Empty Twt: %s", line)
	}

	return twt, err
}

type lextwtManager struct{}

func (*lextwtManager) DecodeJSON(b []byte) (types.Twt, error) { return DecodeJSON(b) }
func (*lextwtManager) ParseLine(line string, twter types.Twter) (twt types.Twt, err error) {
	return ParseLine(line, twter)
}
func (*lextwtManager) ParseFile(r io.Reader, twter types.Twter) (types.TwtFile, error) {
	return ParseFile(r, twter)
}
func (*lextwtManager) MakeTwt(twter types.Twter, ts time.Time, text string) types.Twt {
	dt := NewDateTime(ts, "")
	elems, err := ParseText(text)
	if err != nil {
		return types.NilTwt
	}

	twt := NewTwt(twter, dt, elems...)
	if err != nil {
		return types.NilTwt
	}

	return twt
}

type lextwtFile struct {
	twter    types.Twter
	twts     types.Twts
	comments Comments
}

var _ types.TwtFile = (*lextwtFile)(nil)

func NewTwtFile(twter types.Twter, comments Comments, twts types.Twts) *lextwtFile {
	return &lextwtFile{twter, twts, comments}
}
func (r *lextwtFile) Twter() types.Twter { return r.twter }
func (r *lextwtFile) Info() types.Info   { return r.comments }
func (r *lextwtFile) Twts() types.Twts   { return r.twts }

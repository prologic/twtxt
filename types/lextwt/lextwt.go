package lextwt

// EBNF
// lower = rune, Inital = []rune, CAPS = Element
// ```
// eof     = EOF ;
// illegal =  0 ;
// any     = ? any unicode excluding eof or illegal ? ;
//
// sp      = " " ;
// nl      = "\n" ;
// tab     = "\t" ;
// ls      = "\u2028" ;
//
// term       = EOF | 0 ;
// Space      = { sp }, !( nl | tab | ls ) | term ;
//
// digit   = "0" | "1" | "2" | "3" | "4" | "5" | "6" | "7" | "8" | "9" ;
// Number  = { digit }, !( digit | term ) ;
//
// colon   = ":" ;
// dot     = "." ;
// hyphen  = "-" ;
// plus    = "+" ;
// t       = "T" ;
// z       = "Z" ;
// DATE    = (* year *) Number, hyphen, (* month *) Number, hyphen, (* day *) Number, t, (* hour *) Number, colon, (* minute *) Number,
//           [ colon, (* second *) Number, [ dot, (* nanosec *) Number] ],
//           [ z | (plus | hyphen, (* tzhour *) Number, [ colon, (* tzmin *) Number ] ) ] ;
//
// String  = { any }, !( ? if comment ( "=" | nl ) else ( sp | amp | hash | lt | gt | ls | nl ) ? | term ) ;
// TEXT    = { String | Space | ls } ;
//
// Hash    = "#" ;
// Equal   = "=" ;
// Keyval  = String, equal, String ;
//
// COMMENT = hash, { Space | String } | Keyval ;
//
// amp     = "@" ;
// gt      = ">" ;
// lt      = "<" ;
// MENTION = amp, lt, [ String, Space ], String , gt ;
// TAG     = hash, lt, String, [ Space, String ], gt ;
// PLINK  = lt, String, [ Hash, String ], gt ;
//
// lp      = "(" ;
// rp      = ")" ;
// SUBJECT = lp, TAG | TEXT, rp ;
//
// bang    = "!" ;
// lb      = "[" ;
// rb      = "]" ;
// LINK    = lb, TEXT, rb, lp, TEXT, rp ;
// MEDIA   = bang, lb, [ TEXT ], rb, lp, TEXT, rp ;
//
// TWT     = DATE, tab, [ { MENTION }, [ SUBJECT ] ], { ( Space, MENTION ) | ( Space,  TAG ) | TEXT | LINK | MEDIA | PLINK }, term;
// ```

import (
	"encoding/base32"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/apex/log"
	"github.com/jointwt/twtxt/types"
	"golang.org/x/crypto/blake2b"
)

// ParseFile and return time & count limited twts + comments
func ParseFile(r io.Reader, twter types.Twter) (types.TwtFile, error) {

	f := &lextwtFile{twter: twter}

	nLines, nErrors := 0, 0

	lexer := NewLexer(r)
	parser := NewParser(lexer)
	parser.SetTwter(twter)

	for !parser.IsEOF() {
		elem := parser.ParseLine()

		nLines++

		switch e := elem.(type) {
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
func init() {
	gob.Register(&Twt{})
}

// Lexer

type lexer struct {
	r io.Reader

	// simple ring buffer to xlate bytes to runes.
	rune rune
	last rune
	buf  []byte
	pos  int
	rpos int
	fpos int
	size int

	// state to assist token state machine
	linePos int
	lineNum int
	mode    lexerMode

	// Current token buffer.
	Token   TokType
	Literal []rune
}

type lexerMode int

// LexerModes
const (
	lmDefault lexerMode = iota
	lmDate
	lmComment
	lmEOF
)

// NewLexer tokenizes input for parser.
func NewLexer(r io.Reader) *lexer {
	l := &lexer{
		r:       r,
		buf:     make([]byte, 4096),    // values lower than 2k seem to limit throughput.
		Literal: make([]rune, 0, 1024), // an all text twt would default to be 288 runes. set to ~4x but will grow if needed.
	}
	l.readRune() // prime the lexer buffer.
	return l
}

// EOF represents an end of file.
const EOF rune = -(iota + 1)

// TokType passed to parser.
type TokType string

// TokType values
const (
	TokILLEGAL TokType = "ILLEGAL" // Illegal UTF8
	TokEOF     TokType = "EOF"     // End-of-File

	TokNUMBER TokType = "NUMBER" // Digit 0-9
	TokLS     TokType = "LS"     // Unicode Line Separator
	TokNL     TokType = "NL"     // New Line
	TokSTRING TokType = "STRING" // String
	TokCODE   TokType = "CODE"   // Code Block
	TokSPACE  TokType = "SPACE"  // White Space
	TokTAB    TokType = "TAB"    // Tab
	TokSCHEME TokType = "://"    // URL Scheme

	TokCOLON  TokType = ":"
	TokHYPHEN TokType = "-"
	TokDOT    TokType = "."
	TokPLUS   TokType = "+"
	TokT      TokType = "T"
	TokZ      TokType = "Z"

	TokHASH  TokType = "#"
	TokEQUAL TokType = "="

	TokAMP    TokType = "@"
	TokLT     TokType = "<"
	TokGT     TokType = ">"
	TokLPAREN TokType = "("
	TokRPAREN TokType = ")"
	TokLBRACK TokType = "["
	TokRBRACK TokType = "]"
	TokBANG   TokType = "!"
	TokSLASH  TokType = `\`
)

// // Tested using int8 for TokenType -1 debug +0 memory/performance
// type TokType int8
//
// // TokType values
// const (
// 	TokILLEGAL TokType = iota + 1 // Illegal UTF8
// 	TokEOF                        // End-of-File
//
// 	TokNUMBER  // Digit 0-9
// 	TokLS     // Unicode Line Separator
// 	TokNL     // New Line
// 	TokSTRING // String
// 	TokSPACE  // White Space
// 	TokTAB    // Tab
//
// 	TokAMP
// 	TokCOLON
// 	TokDOT
// 	TokHASH
// 	TokHYPHEN
// 	TokGT
// 	TokLT
// 	TokPLUS
// 	TokT
// 	TokZ
// )

// NextRune decode next rune in buffer
func (l *lexer) NextRune() bool {
	l.readRune()
	return l.rune != EOF && l.rune != 0
}

// NextTok decode next token. Returns true on success
func (l *lexer) NextTok() bool {
	l.Literal = l.Literal[:0]

	switch l.rune {
	case ' ':
		l.Token = TokSPACE
		l.loadSpace()
		return true
	case '\u2028':
		l.loadRune(TokLS)
		return true
	case '\t':
		l.mode = lmDefault
		l.loadRune(TokTAB)
		return true
	case '\n':
		l.mode = lmDefault
		l.loadRune(TokNL)
		return true
	case EOF:
		l.mode = lmDefault
		l.loadRune(TokEOF)
		return false
	case 0:
		l.mode = lmDefault
		l.loadRune(TokILLEGAL)
		return false
	}

	switch l.mode {
	case lmEOF:
		l.loadRune(TokEOF)
		return false

	case lmDefault:
		// Special modes at line position 0.
		if l.linePos == 0 {
			switch {
			case l.rune == '#':
				l.mode = lmComment
				return l.NextTok()

			case '0' <= l.rune && l.rune <= '9':
				l.mode = lmDate
				return l.NextTok()
			}
		}

		switch l.rune {
		case '@':
			l.loadRune(TokAMP)
			return true
		case '#':
			l.loadRune(TokHASH)
			return true
		case '<':
			l.loadRune(TokLT)
			return true
		case '>':
			l.loadRune(TokGT)
			return true
		case '(':
			l.loadRune(TokLPAREN)
			return true
		case ')':
			l.loadRune(TokRPAREN)
			return true
		case '[':
			l.loadRune(TokLBRACK)
			return true
		case ']':
			l.loadRune(TokRBRACK)
			return true
		case '!':
			l.loadRune(TokBANG)
			return true
		case '\\':
			l.loadRune(TokSLASH)
			return true
		case '`':
			l.loadCode()
			return true
		case ':':
			l.loadScheme()
			return true
		default:
			l.loadString(" @#!:`<>()[]\u2028\n")
			return true
		}

	case lmDate:
		switch l.rune {
		case ':':
			l.loadRune(TokCOLON)
			return true
		case '-':
			l.loadRune(TokHYPHEN)
			return true
		case '+':
			l.loadRune(TokPLUS)
			return true
		case '.':
			l.loadRune(TokDOT)
			return true
		case 'T':
			l.loadRune(TokT)
			return true
		case 'Z':
			l.loadRune(TokZ)
			return true

		default:
			if '0' <= l.rune && l.rune <= '9' {
				l.loadNumber()
				return true
			}
		}

	case lmComment:
		switch l.rune {
		case '#':
			l.loadRune(TokHASH)
			return true
		case '=':
			l.loadRune(TokEQUAL)
			return true

		default:
			l.loadString("=\n")
			return true
		}
	}

	l.loadRune(TokILLEGAL)
	return false
}

// Rune current rune from ring buffer. (only used by unit tests)
func (l *lexer) Rune() rune {
	return l.rune
}

// GetTok return latest decoded token. (only used by unit tests)
func (l *lexer) GetTok() Token {
	return Token{l.Token, l.Literal}
}

func (l *lexer) readBuf() {
	size, err := l.r.Read(l.buf[l.pos:])
	if err != nil && size == 0 {
		l.size = 0
		return
	}
	l.size += size
}

func (l *lexer) readRune() {
	if l.rune == EOF {
		return
	}
	l.last = l.rune

	// If empty init the buffer.
	if l.size-l.pos <= 0 {
		l.pos, l.size = 0, 0
		l.readBuf()
	}
	if l.size-l.pos <= 0 {
		l.rune = EOF
		return
	}

	// if not enough bytes left shift and fill.
	// After testing the DecodeRune internally calls FullRune
	// So it is better to optimistically attempt a decode and
	// replenish the buffer if that fails.
	var size int
	// if !utf8.FullRune(l.buf[l.pos:]) {
	// 	copy(l.buf[:], l.buf[l.pos:l.size])
	// 	l.pos = l.size - l.pos
	// 	l.size = l.pos
	// 	l.readBuf()
	// 	l.pos = 0
	// }
	// if !utf8.FullRune(l.buf[l.pos:]) {
	// 	l.rune = EOF
	// 	return
	// }

	l.rune, size = utf8.DecodeRune(l.buf[l.pos:])
	if l.rune == utf8.RuneError && size == 0 {
		copy(l.buf[:], l.buf[l.pos:l.size])
		l.pos = l.size - l.pos
		l.size = l.pos
		l.readBuf()
		l.pos = 0

		l.rune, size = utf8.DecodeRune(l.buf[l.pos:])
	}

	l.pos += size
	l.rpos = l.fpos
	l.fpos += size

	if l.last == '\n' {
		l.last = 0
		l.lineNum++
		l.linePos = 0
	}
	if l.last != 0 {
		l.linePos++
	}
}

func (l *lexer) loadRune(tok TokType) {
	l.Token = tok
	l.Literal = append(l.Literal, l.rune)
	l.readRune()
}

func (l *lexer) loadNumber() {
	l.Token = TokNUMBER
	for strings.ContainsRune("0123456789", l.rune) {
		l.Literal = append(l.Literal, l.rune)
		l.readRune()
	}
}

func (l *lexer) loadString(notaccept string) {
	l.Token = TokSTRING
	for !(strings.ContainsRune(notaccept, l.rune) || l.rune == 0 || l.rune == EOF) {
		l.Literal = append(l.Literal, l.rune)
		l.readRune()
	}
}
func (l *lexer) loadScheme() {
	l.Token = TokSTRING
	l.Literal = append(l.Literal, l.rune)
	l.readRune()
	if l.rune == '/' {
		l.Literal = append(l.Literal, l.rune)
		l.readRune()
		if l.rune == '/' {
			l.Token = TokSCHEME
			l.Literal = append(l.Literal, l.rune)
			l.readRune()
		}
	}
}
func (l *lexer) loadCode() {
	l.Token = TokCODE
	l.Literal = append(l.Literal, l.rune)
	l.readRune()
	block := false
	if l.rune == '`' {
		l.Literal = append(l.Literal, l.rune)
		l.readRune()
		if l.rune != '`' {
			return // only two ends the token.
		}

		block = true
		l.Literal = append(l.Literal, l.rune)
		l.readRune()
	}

	for !(l.rune == '`' || l.rune == 0 || l.rune == EOF || l.rune == '\n') {
		l.Literal = append(l.Literal, l.rune)
		l.readRune()

		if block && l.rune == '`' {
			l.Literal = append(l.Literal, l.rune)
			l.readRune()
			if l.rune == '`' {
				l.Literal = append(l.Literal, l.rune)
				l.readRune()
				if l.rune == '`' {
					l.Literal = append(l.Literal, l.rune)
					l.readRune()
					return
				}
			}
		}
	}

	l.Literal = append(l.Literal, l.rune)
	l.readRune()
}

func (l *lexer) loadSpace() {
	l.Token = TokSPACE
	for !(strings.ContainsRune("\t\n\u2028", l.rune) || l.rune == 0 || l.rune == EOF) && unicode.IsSpace(l.rune) {
		l.Literal = append(l.Literal, l.rune)
		l.readRune()
	}
}

// Parser

type parser struct {
	l       *lexer
	curTok  Token
	curPos  int
	nextTok Token
	nextPos int

	twter types.Twter

	lit   []rune
	frame []int

	errs []error
}

func (p *parser) Literal() string    { return string(p.lit[p.pos():]) }
func (p *parser) append(lis ...rune) { p.lit = append(p.lit, lis...) }
func (p *parser) pos() int {
	if len(p.frame) == 0 {
		return 0
	}
	return p.frame[len(p.frame)-1]
}
func (p *parser) push() {
	p.frame = append(p.frame, len(p.lit))
}
func (p *parser) pop() {
	if len(p.frame) == 0 {
		return
	}
	p.frame = p.frame[:len(p.frame)-1]
}
func (p *parser) clear() {
	p.frame = p.frame[:0]
	p.lit = p.lit[:0]
}

type Token struct {
	Type    TokType
	Literal []rune
}

func (t Token) String() string {
	return fmt.Sprintf("%s[%s]", t.Type, string(t.Literal))
}

func NewParser(l *lexer) *parser {
	p := &parser{
		l: l,

		// as tokens are read they are appended here and stored in the resulting Elem.
		// the buffer is here so text can be recovered in the event a menton/tag fails to fully parse.
		// and to limit memory allocs.
		lit: make([]rune, 0, 1024),
	}

	// Prime the parser queue
	p.next()
	p.next()

	return p
}

func (p *parser) SetTwter(twter types.Twter) {
	p.twter = twter
}

// ParseLine from tokens
// Forms parsed:
//   #... -> ParseComment
//   [digit]... -> ParseTwt
func (p *parser) ParseLine() Line {
	var e Line

	switch p.curTok.Type {
	case TokHASH:
		e = p.ParseComment()
	case TokNUMBER:
		e = p.ParseTwt()
	default:
		p.nextLine()
	}
	if !(p.expect(TokNL) || p.expect(TokEOF)) {
		return nil
	}
	p.next()
	p.clear()

	return e
}

// ParseElem from tokens
// Forms parsed:
//   #... -> ParseTag
//   @... -> ParseMention
//   [Text] -> ParseText
// If the parse fails for Tag or Mention it will fallback to Text
func (p *parser) ParseElem() Elem {
	var e Elem

	switch p.curTok.Type {
	case TokLBRACK, TokBANG, TokLT:
		e = p.ParseLink()
	case TokCODE:
		e = p.ParseCode()
	case TokLS:
		e = p.ParseLineSeparator()
	case TokLPAREN:
		e = p.ParseSubject()
	case TokHASH:
		e = p.ParseTag()
	case TokAMP:
		e = p.ParseMention()
	case TokNL, TokEOF:
		return nil
	default:
		if p.curTokenIs(TokSTRING) && p.peekTokenIs(TokSCHEME) {
			e = p.ParseLink()
		} else {
			e = p.ParseText()
		}
	}

	// If parsing above failed convert to Text
	if e == nil || e.IsNil() {
		e = p.ParseText()
	}

	return e
}

// ParseComment from tokens
// Forms parsed:
//   # comment
//   # key = value
func (p *parser) ParseComment() *Comment {
	if !p.curTokenIs(TokHASH) {
		return nil
	}

	p.append(p.curTok.Literal...)

	isKeyVal := false
	var label string
	var value []rune
	for !p.nextTokenIs(TokNL, TokEOF) {
		p.next()
		p.append(p.curTok.Literal...)

		if isKeyVal && p.curTokenIs(TokSTRING) {
			value = append(value, p.curTok.Literal...)
		}

		if !isKeyVal && p.curTokenIs(TokSTRING) && p.peekTokenIs(TokEQUAL) {
			isKeyVal = true
			label = strings.TrimSpace(string(p.curTok.Literal))
			p.next()
			p.lit = append(p.lit, p.curTok.Literal...)
			p.next()
			p.lit = append(p.lit, p.curTok.Literal...)
		}
	}
	return NewCommentValue(p.Literal(), label, strings.TrimSpace(string(value)))
}

// ParseTwt from tokens
// Forms parsed:
//   [Date]\t... -> ParseElem (will consume all elems till end of line/file.)
func (p *parser) ParseTwt() *Twt {
	twt := &Twt{twter: p.twter}

	if !p.expect(TokNUMBER) {
		return nil
	}
	twt.pos = p.curPos
	twt.dt = p.ParseDateTime()
	if twt.dt == nil {
		return nil
	}
	p.push()

	if !p.expect(TokTAB) {
		return nil
	}
	p.append(p.curTok.Literal...)
	p.push()

	p.next()

	for elem := p.ParseElem(); elem != nil; elem = p.ParseElem() {
		p.push()
		twt.append(elem)
	}

	return twt
}

// ParseDateTime from tokens
// Forms parsed:
//   YYYY-MM-DD'T'HH:mm[:ss[.nnnnnnnn]]('Z'|('+'|'-')th[:tm])
//   YYYY = year, MM = month, DD = day, HH = 24hour, mm = minute, ss = sec, nnnnnnnn = nsec, th = timezone hour, tm = timezone minute
func (p *parser) ParseDateTime() *DateTime {
	var ok bool
	var year, month, day, hour, min, sec, nsec, sign, tzhour, tzmin int
	loc := time.UTC

	// Year
	p.append(p.curTok.Literal...)
	if year, ok = p.parseDigit(); !ok {
		return nil
	}

	// Hyphen
	p.append(p.curTok.Literal...)
	if !(p.expect(TokHYPHEN) && p.expectNext(TokNUMBER)) {
		return nil
	}

	// Month
	p.append(p.curTok.Literal...)
	if month, ok = p.parseDigit(); !ok {
		return nil
	}

	// Hyphen
	p.append(p.curTok.Literal...)
	if !(p.expect(TokHYPHEN) && p.expectNext(TokNUMBER)) {
		return nil
	}

	// Day
	p.append(p.curTok.Literal...)
	if day, ok = p.parseDigit(); !ok {
		return nil
	}

	// T
	p.append(p.curTok.Literal...)
	if !(p.expect(TokT) && p.expectNext(TokNUMBER)) {
		return nil
	}

	// Hour
	p.append(p.curTok.Literal...)
	if hour, ok = p.parseDigit(); !ok {
		return nil
	}

	// Colon
	p.append(p.curTok.Literal...)
	if !(p.expect(TokCOLON) && p.expectNext(TokNUMBER)) {
		return nil
	}

	// Minute
	p.append(p.curTok.Literal...)
	if min, ok = p.parseDigit(); !ok {
		return nil
	}

	// Optional Second
	if p.curTokenIs(TokCOLON) {
		p.append(p.curTok.Literal...)
		if !p.expectNext(TokNUMBER) {
			return nil
		}

		// Second
		p.append(p.curTok.Literal...)
		if sec, ok = p.parseDigit(); !ok {
			return nil
		}
	}

	// Optional NSec
	if p.curTokenIs(TokDOT) {
		p.append(p.curTok.Literal...)
		if !p.expectNext(TokNUMBER) {
			return nil
		}

		// NSecond
		p.append(p.curTok.Literal...)
		if nsec, ok = p.parseDigit(); !ok {
			return nil
		}
	}

	// UTC Timezone
	if p.curTokenIs(TokZ) {
		p.append(p.curTok.Literal...)
		p.next()

	} else if p.curTokenIs(TokPLUS) || p.curTokenIs(TokHYPHEN) {
		sign = 1
		tzfmt := "UTC+%02d%02d"

		p.append(p.curTok.Literal...)
		if p.curTokenIs(TokHYPHEN) {
			tzfmt = "UTC-%02d%02d"
			sign = -1
		}
		// TZHour
		if !p.expectNext(TokNUMBER) {
			return nil
		}
		p.append(p.curTok.Literal...)
		if tzhour, ok = p.parseDigit(); !ok {
			return nil
		}

		if tzhour > 24 {
			tzmin = tzhour % 100
			tzhour = tzhour / 100
		}

		// Optional tzmin with colon
		if p.curTokenIs(TokCOLON) {
			p.append(p.curTok.Literal...)
			if !p.expectNext(TokNUMBER) {
				return nil
			}

			// TZMin
			p.append(p.curTok.Literal...)
			if tzmin, ok = p.parseDigit(); !ok {
				return nil
			}
		}

		loc = time.FixedZone(fmt.Sprintf(tzfmt, tzhour, tzmin), sign*tzhour*3600+tzmin*60)
	}

	return &DateTime{dt: time.Date(year, time.Month(month), day, hour, min, sec, nsec, loc), lit: p.Literal()}
}

// ParseMention from tokens
// Forms parsed:
//   @name
//   @name@domain
//   @<target>
//   @<name target>
//   @<name@domain>
//   @<name@domain target>
func (p *parser) ParseMention() *Mention {
	m := &Mention{}

	// form: @nick
	if p.curTokenIs(TokAMP) && p.peekTokenIs(TokSTRING) {
		p.append(p.curTok.Literal...) // @
		p.next()

		m.name = string(p.curTok.Literal)

		p.append(p.curTok.Literal...)
		p.next()

		if p.curTokenIs(TokAMP) && p.peekTokenIs(TokSTRING) {
			p.append(p.curTok.Literal...)
			p.next()

			m.domain = string(p.curTok.Literal)

			p.append(p.curTok.Literal...)
			p.next()
		}

		m.lit = p.Literal()
		return m
	}

	// forms: @<...>
	if p.curTokenIs(TokAMP) && p.peekTokenIs(TokLT) {
		p.append(p.curTok.Literal...) // @
		p.next()

		p.append(p.curTok.Literal...) // <
		p.next()

		// form: @<nick scheme://example.com>
		if p.curTokenIs(TokSTRING) && p.peekTokenIs(TokSPACE) {
			m.name = string(p.curTok.Literal)

			p.append(p.curTok.Literal...) // string
			p.next()
			if !p.curTokenIs(TokSPACE) {
				return nil
			}
		}

		// form: @<nick@domain scheme://example.com>
		if p.curTokenIs(TokSTRING) && p.peekTokenIs(TokAMP) {
			m.name = string(p.curTok.Literal)

			p.append(p.curTok.Literal...) // string
			p.next()

			p.append(p.curTok.Literal...) // @
			p.next()

			m.domain = string(p.curTok.Literal)

			p.append(p.curTok.Literal...)
			p.next()
			if !p.curTokenIs(TokSPACE) {
				return nil
			}
		}

		if p.curTokenIs(TokSPACE) {
			p.append(p.curTok.Literal...)
			p.next()
		}

		// form: #<[...]scheme://example.com>
		if p.curTokenIs(TokSTRING) && p.peekTokenIs(TokSCHEME) {
			p.push()
			l := p.ParseLink()
			p.pop()

			if l == nil {
				return nil // bad url
			}

			m.target = l.target
		}

		if !p.curTokenIs(TokGT) {
			return nil
		}
		p.append(p.curTok.Literal...) // >
		p.next()

		m.lit = p.Literal()

		return m
	}

	return nil
}

// ParseTag from tokens
// Forms parsed:
//   #tag
//   #<target>
//   #<tag target>
func (p *parser) ParseTag() *Tag {
	tag := &Tag{}

	// form: #tag
	if p.curTokenIs(TokHASH) && p.peekTokenIs(TokSTRING) {
		p.append(p.curTok.Literal...) // #
		p.next()

		p.append(p.curTok.Literal...) // string
		tag.lit = p.Literal()
		tag.tag = string(p.curTok.Literal)

		p.next()

		return tag
	}

	// form: #<...>
	if p.curTokenIs(TokHASH) && p.peekTokenIs(TokLT) {
		p.append(p.curTok.Literal...) // #
		p.next()

		p.append(p.curTok.Literal...) // <
		p.next()

		// form: #<tag scheme://example.com>
		if p.curTokenIs(TokSTRING) && p.peekTokenIs(TokSPACE) {
			p.append(p.curTok.Literal...) // string
			tag.tag = string(p.curTok.Literal)
			p.next()

			p.append(p.curTok.Literal...) // space
			p.next()
		}

		// form: #<scheme://example.com>
		if p.curTokenIs(TokSTRING) && p.peekTokenIs(TokSCHEME) {
			p.push()
			l := p.ParseLink()
			p.pop()

			if l == nil {
				return nil // bad url
			}

			tag.target = l.target
		}

		if !p.curTokenIs(TokGT) {
			return nil
		}

		p.append(p.curTok.Literal...) // >
		p.next()

		tag.lit = p.Literal()

		return tag
	}

	return nil
}

// ParseSubject from tokens
// Forms parsed:
//   (#tag)
//   (#<target>)
//   (#<tag target>)
//   (re: something)
func (p *parser) ParseSubject() *Subject {
	subject := &Subject{}

	p.append(p.curTok.Literal...) // (
	p.next()

	// form: (#tag)
	if p.curTokenIs(TokHASH) {
		p.push()
		subject.tag = p.ParseTag()
		p.pop()

		if !p.curTokenIs(TokRPAREN) {
			return nil
		}
		p.append(p.curTok.Literal...) // )
		p.next()

		return subject
	}

	// form: (text)
	if !p.curTokenIs(TokRPAREN) {
		p.push()
		subject.subject = p.ParseText().String()
		p.pop()

		if !p.curTokenIs(TokRPAREN) {
			return nil
		}

		p.append(p.curTok.Literal...) // )
		p.next()

		return subject
	}

	return nil
}

// ParseText from tokens.
// Forms parsed:
//   combination of string and space tokens.
func (p *parser) ParseText() *Text {
	p.append(p.curTok.Literal...)
	p.next()

	for p.curTokenIs(TokSTRING, TokSPACE) ||
		// We don't want to parse an email address or link accidentally as a mention or tag. So check if it is preceded with a space.
		(p.curTokenIs(TokHASH, TokAMP, TokLT, TokLPAREN) && (len(p.lit) == 0 || !unicode.IsSpace(p.lit[len(p.lit)-1]))) {

		// if it looks like a link break out.
		if p.curTokenIs(TokSTRING) && p.peekTokenIs(TokSCHEME) {
			break
		}

		p.append(p.curTok.Literal...)
		p.next()
	}

	txt := &Text{p.Literal()}

	return txt
}

func (p *parser) ParseLineSeparator() Elem {
	p.append(p.curTok.Literal...)
	p.next()
	return LineSeparator
}

// ParseLink from tokens.
// Forms parsed:
//   scheme://example.com
//	 <scheme://example.com>
//   [a link](scheme://example.com)
//   ![a image](scheme://example.com/img.png)
//
func (p *parser) ParseLink() *Link {
	link := &Link{linkType: LinkStandard}

	if p.curTokenIs(TokSTRING) && p.peekTokenIs(TokSCHEME) {
		link.linkType = LinkNaked

		p.append(p.curTok.Literal...) // scheme
		p.next()

		p.append(p.curTok.Literal...) // link text
		for !p.nextTokenIs(TokGT, TokRPAREN, TokSPACE, TokNL, TokLS, TokEOF) {
			p.next()
			p.append(p.curTok.Literal...) // link text

			// Allow excaped chars to not close.
			if p.curTokenIs(TokSLASH) {
				p.next()
				p.append(p.curTok.Literal...) // text
			}
		}

		link.target = p.Literal()

		return link
	}

	// Plain Link
	if p.curTokenIs(TokLT) && p.peekTokenIs(TokSTRING) {
		link.linkType = LinkPlain
		p.append(p.curTok.Literal...) // <
		p.next()

		p.push()
		l := p.ParseLink()
		p.pop()

		if l == nil {
			return nil
		}
		if !p.curTokenIs(TokGT) {
			return nil
		}

		p.append(p.curTok.Literal...) // >
		p.next()
		link.target = l.target

		return link
	}

	// Media Link
	if p.curTokenIs(TokBANG) && p.peekTokenIs(TokLBRACK) {
		link.linkType = LinkMedia
		p.append(p.curTok.Literal...) // !
		p.next()
	}

	if !p.curTokenIs(TokLBRACK) {
		return nil
	}

	// Parse Text
	p.append(p.curTok.Literal...) // [
	p.next()

	if !p.curTokenIs(TokRBRACK) {
		p.push()
		p.append(p.curTok.Literal...) // text
		p.next()

		for !p.curTokenIs(TokRBRACK, TokLBRACK, TokRPAREN, TokLPAREN, TokEOF) {
			p.append(p.curTok.Literal...) // text
			p.next()

			// Allow excaped chars to not close.
			if p.curTokenIs(TokSLASH) {
				p.append(p.curTok.Literal...) // text
				p.next()
			}
		}
		link.text = p.Literal()
	}

	if !p.curTokenIs(TokRBRACK) {
		return nil
	}

	p.append(p.curTok.Literal...) // ]
	p.next()

	// Parse Target
	if p.curTokenIs(TokLPAREN) && !p.peekTokenIs(TokRPAREN) {
		p.append(p.curTok.Literal...) // (
		p.next()

		p.push()
		l := p.ParseLink()
		p.pop()

		if l == nil {
			return nil
		}

		link.target = l.target

		if !p.curTokenIs(TokRPAREN) {
			return nil
		}

		p.append(p.curTok.Literal...) // )
		p.next()

		return link
	}

	return nil
}

// ParseCode from tokens
// Forms parsed:
//   `inline code`
//   ```
//   block code
//   ```
func (p *parser) ParseCode() *Code {
	code := &Code{}
	p.append(p.curTok.Literal...) // )

	lit := p.Literal()
	if len(lit) >= 6 && lit[:3] == "```" && lit[len(lit)-3:] == "```" {
		code.codeType = CodeBlock
		code.lit = string(lit[3 : len(lit)-3])

		p.next()

		return code
	}

	code.codeType = CodeInline
	code.lit = string(lit[1 : len(lit)-1])

	p.next()

	return code
}

func (p *parser) Errs() ListError {
	if len(p.errs) == 0 {
		return nil
	}
	return p.errs
}

type ListError []error

func (e ListError) Error() string {
	var b strings.Builder
	for _, err := range e {
		b.WriteString(err.Error())
		b.WriteRune('\n')
	}
	return b.String()
}

// Parser evaluation functions.

func (p *parser) IsEOF() bool {
	return p.curTokenIs(TokEOF)
}

// next promotes the next token and loads a new one.
// the parser keeps two buffers to store tokens and alternates them here.
func (p *parser) next() {
	p.curPos, p.nextPos = p.nextPos, p.l.rpos
	p.curTok, p.nextTok = p.nextTok, p.curTok
	p.nextTok.Literal = p.nextTok.Literal[:0]
	p.l.NextTok()
	p.nextTok.Type = p.l.Token
	p.nextTok.Literal = append(p.nextTok.Literal, p.l.Literal...)
}

func (p *parser) nextLine() {
	for !p.curTokenIs(TokNL, TokEOF) {
		p.next()
	}
}

// curTokenIs returns true if any of provited TokTypes match current token.
func (p *parser) curTokenIs(tokens ...TokType) bool {
	for _, t := range tokens {
		if p.curTok.Type == t {
			return true
		}
	}
	return false
}

// peekTokenIs returns true if any of provited TokTypes match next token.
func (p *parser) peekTokenIs(tokens ...TokType) bool {
	for _, t := range tokens {
		if p.nextTok.Type == t {
			return true
		}
	}
	return false
}

// nextTokenIs returns true if any of provited TokTypes match next token and reads next token. to next token.
func (p *parser) nextTokenIs(tokens ...TokType) bool {
	if p.peekTokenIs(tokens...) {
		p.next()
		return true
	}

	return false
}

// expect returns true if the current token matches. adds error if not.
// Need to come up with a good proxy for failed parsing of a twtxt line.
// Current mode is to treat failed elements as text.
func (p *parser) expect(t TokType) bool {
	// return p.curTokenIs(t)

	if p.curTokenIs(t) {
		return true
	}

	p.addError(fmt.Errorf("%w: expected current %v, found %v", ErrParseToken, t, p.curTok.Type))
	return false
}

// expectNext returns true if the current token matches and reads to next token. adds error if not.
func (p *parser) expectNext(t TokType) bool {
	// return p.peekTokenIs(t)

	if p.peekTokenIs(t) {
		p.next()
		return true
	}

	p.addError(fmt.Errorf("%w: expected next %v, found %v", ErrParseToken, t, p.nextTok.Type))
	return false
}

// parseDigit converts current token to int. adds error if fails.
func (p *parser) parseDigit() (int, bool) {
	if !p.curTokenIs(TokNUMBER) {
		p.addError(fmt.Errorf("%w: expected digit, found %T", ErrParseToken, p.curTok.Type))
		return 0, false
	}

	i, err := strconv.Atoi(string(p.curTok.Literal))

	p.addError(err)
	p.next()

	return i, err == nil
}

// addError to parser.
func (p *parser) addError(err error) {
	if err != nil {
		p.errs = append(p.errs, err)
	}
}

var ErrParseElm = errors.New("error parsing element")
var ErrParseToken = errors.New("error parsing digit")

// Elem AST structs

type Elem interface {
	IsNil() bool        // A typed nil will fail `elem == nil` We need to unbox to test.
	Literal() string    // value as read from input.
	Markdown() string   // format to markdown.
	FormatText() string // format to write to disk.
	Clone() Elem        // clone element.

	fmt.Stringer // alias for Literal() for printing.
}

type Line interface {
	IsNil() bool     // A typed nil will fail `elem == nil` We need to unbox to test.
	Literal() string // value as read from input.
	fmt.Stringer     // alias for Literal() for printing.
}

type Comment struct {
	comment string
	key     string
	value   string
}

var _ Line = (*Comment)(nil)

func NewComment(comment string) *Comment {
	return &Comment{comment: comment}
}
func NewCommentValue(comment, key, value string) *Comment {
	return &Comment{comment, key, value}
}
func (n Comment) IsNil() bool     { return n.comment == "" }
func (n Comment) Literal() string { return n.comment + "\n" }
func (n Comment) String() string  { return n.Literal() }
func (n Comment) Key() string     { return n.key }
func (n Comment) Value() string   { return n.value }

type Comments []*Comment

var _ types.KV = Comments{}

func (lis Comments) String() string {
	var b strings.Builder
	for _, line := range lis {
		b.WriteString(line.Literal())
	}
	return b.String()
}

func (lis Comments) GetN(key string, n int) (types.Value, bool) {
	idx := make([]int, 0, len(lis))

	for i := range lis {
		if n == 0 && key == lis[i].key {
			return lis[i], true
		}

		if key == lis[i].key {
			idx = append(idx, i)
		}

		if n == len(idx) && key == lis[i].key {
			return lis[i], true
		}
	}

	if n < 0 && -n < len(idx) {
		return lis[idx[len(idx)+n]], true
	}

	return nil, false
}

func (lis Comments) GetAll(prefix string) []types.Value {
	nlis := make([]types.Value, 0, len(lis))

	for i := range lis {
		if lis[i].key == "" {
			continue
		}

		if strings.HasPrefix(lis[i].key, prefix) {
			nlis = append(nlis, lis[i])
		}
	}

	return nlis
}

func (lis Comments) Followers() []types.Twter {
	flis := lis.GetAll("follow")
	nlis := make([]types.Twter, 0, len(flis))

	for _, o := range flis {
		sp := strings.Fields(o.Value())
		if len(sp) < 2 {
			continue
		}
		nlis = append(nlis, types.Twter{Nick: sp[0], URL: sp[1]})
	}

	return nlis
}

type DateTime struct {
	lit string

	dt time.Time
}

// var _ Elem = (*DateTime)(nil)

func NewDateTime(dt time.Time) *DateTime {
	return &DateTime{dt: dt, lit: dt.Format(time.RFC3339)}
}
func (n *DateTime) CloneDateTime() *DateTime {
	if n == nil {
		return nil
	}
	return &DateTime{
		n.lit, n.dt,
	}
}
func (n *DateTime) IsNil() bool { return n == nil }
func (n *DateTime) Literal() string {
	if n == nil {
		return ""
	}
	return n.lit
}
func (n *DateTime) String() string { return n.Literal() }
func (n *DateTime) DateTime() time.Time {
	if n == nil {
		return time.Time{}
	}
	return n.dt
}

type Mention struct {
	lit string

	name   string
	domain string
	target string
	url    *url.URL
	err    error
}

var _ Elem = (*Mention)(nil)
var _ types.TwtMention = (*Mention)(nil)

func NewMention(name, target string) *Mention {
	m := &Mention{name: name, target: target}

	switch {
	case name != "" && target == "":
		m.lit = fmt.Sprintf("@%s", name)

	case name != "" && target != "":
		m.lit = fmt.Sprintf("@<%s %s>", name, target)

	case name == "" && target != "":
		m.lit = fmt.Sprintf("@<%s>", target)
	}

	if sp := strings.SplitN(name, "@", 2); len(sp) == 2 {
		m.name = sp[0]
		m.domain = sp[1]
	}

	if m.domain == "" && m.target != "" {
		if url := m.URL(); url != nil {
			m.domain = url.Hostname()
		}
	}

	return m
}
func (n *Mention) Clone() Elem {
	if n == nil {
		return nil
	}
	return &Mention{
		n.lit, n.name, n.domain, n.target, n.url, n.err,
	}
}
func (n *Mention) IsNil() bool        { return n == nil }
func (n *Mention) Twter() types.Twter { return types.Twter{Nick: n.name, URL: n.target} }
func (n *Mention) Literal() string    { return n.FormatText() }
func (n *Mention) String() string     { return n.FormatText() }
func (n *Mention) Name() string       { return n.name }
func (n *Mention) Domain() string {
	if url := n.URL(); n.domain == "" && url != nil {
		n.domain = url.Hostname()
	}
	return n.domain
}
func (n *Mention) Target() string          { return n.target }
func (n *Mention) SetTarget(target string) { n.target, n.url, n.err = target, nil, nil }
func (n *Mention) URL() *url.URL {
	if n.url == nil && n.err == nil {
		n.url, n.err = url.Parse(n.target)
	}
	return n.url
}
func (n *Mention) Err() error {
	n.URL()
	return n.err
}
func (n *Mention) FormatText() string {
	if n.name == "" && n.target != "" {
		return fmt.Sprintf("@<%s>", n.target)
	}

	nick := n.name

	if n.domain != "" {
		nick += "@" + n.domain
	}

	if n.target == "" {
		return fmt.Sprintf("@%s", nick)
	}

	return fmt.Sprintf("@<%s %s>", nick, n.target)
}
func (n *Mention) Markdown() string {
	nick := n.name

	if n.domain != "" {
		nick += "@" + n.domain
	}

	if n.target == "" {
		return fmt.Sprintf("@%s", nick)
	}

	return fmt.Sprintf("[@%s](%s)", nick, n.target)
}

type Tag struct {
	lit string

	tag    string
	target string
	url    *url.URL
	err    error
}

var _ Elem = (*Tag)(nil)
var _ types.TwtTag = (*Tag)(nil)

func NewTag(tag, target string) *Tag {
	m := &Tag{tag: tag, target: target}
	m.lit = m.FormatText()

	return m
}
func (n *Tag) Clone() Elem {
	return n.CloneTag()
}
func (n *Tag) CloneTag() *Tag {
	if n == nil {
		return nil
	}
	return &Tag{
		n.lit, n.tag, n.target, n.url, n.err,
	}
}
func (n *Tag) IsNil() bool     { return n == nil }
func (n *Tag) Literal() string { return n.FormatText() }
func (n *Tag) String() string  { return n.FormatText() }
func (n *Tag) Text() string    { return n.tag }
func (n *Tag) Target() string  { return n.target }
func (n *Tag) SetTarget(target string) {
	if !n.IsNil() {
		n.target, n.url, n.err = target, nil, nil
	}
}
func (n *Tag) Err() error { return n.err }
func (n *Tag) URL() *url.URL {
	if n.url == nil && n.err == nil {
		n.url, n.err = url.Parse(n.target)
	}
	return n.url
}
func (n *Tag) FormatText() string {
	if n.target == "" {
		return fmt.Sprintf("#%s", n.tag)
	}

	if n.tag == "" {
		return fmt.Sprintf("#<%s>", n.target)
	}

	return fmt.Sprintf("#<%s %s>", n.tag, n.target)
}
func (n *Tag) Markdown() string {
	if n.target == "" {
		return fmt.Sprintf("#%s", n.tag)
	}

	if n.tag == "" {
		url := n.URL()
		return fmt.Sprintf("[%s%s](%s)", url.Hostname(), url.Path, n.target)
	}

	return fmt.Sprintf("[#%s](%s)", n.tag, n.target)
}

type Subject struct {
	subject string
	tag     *Tag
}

var _ Elem = (*Subject)(nil)

func NewSubject(text string) *Subject           { return &Subject{subject: text} }
func NewSubjectTag(tag, target string) *Subject { return &Subject{tag: NewTag(tag, target)} }
func (n *Subject) Clone() Elem {
	if n == nil {
		return nil
	}
	return &Subject{
		n.subject,
		n.tag.CloneTag(),
	}
}
func (n *Subject) IsNil() bool { return n == nil }
func (n *Subject) Literal() string {
	if n.tag != nil {
		return "(" + n.tag.Literal() + ")"
	}

	return "(" + n.subject + ")"
}
func (n *Subject) String() string { return n.Literal() }
func (n *Subject) Text() string {
	if n.tag == nil {
		return n.subject
	}
	return n.tag.Literal()
}
func (n *Subject) Tag() types.TwtTag { return n.tag }
func (n *Subject) FormatText() string {
	if n.tag == nil {
		return fmt.Sprintf("(%s)", n.subject)
	}
	return fmt.Sprintf("(%s)", n.tag.FormatText())
}
func (n *Subject) Markdown() string {
	if n.tag == nil {
		return fmt.Sprintf("(%s)", n.subject)
	}
	return fmt.Sprintf("(%s)", n.tag.Markdown())
}

type Text struct {
	lit string
}

var _ Elem = (*Text)(nil)

func NewText(txt string) *Text { return &Text{txt} }
func (n *Text) Clone() Elem {
	if n == nil {
		return nil
	}
	return &Text{n.lit}
}
func (n *Text) IsNil() bool        { return n == nil }
func (n *Text) Literal() string    { return n.lit }
func (n *Text) String() string     { return n.Literal() }
func (n *Text) FormatText() string { return n.Literal() }
func (n *Text) Markdown() string   { return n.Literal() }

type lineSeparator struct{}

var LineSeparator Elem = &lineSeparator{}

func (n *lineSeparator) Clone() Elem        { return LineSeparator }
func (n *lineSeparator) IsNil() bool        { return false }
func (n *lineSeparator) Literal() string    { return "\u2028" }
func (n *lineSeparator) String() string     { return "\n" }
func (n *lineSeparator) FormatText() string { return n.String() }
func (n *lineSeparator) Markdown() string   { return n.String() }

type Link struct {
	linkType LinkType
	text     string
	target   string
}

var _ Elem = (*Link)(nil)

type LinkType int

const (
	LinkStandard LinkType = iota + 1
	LinkMedia
	LinkPlain
	LinkNaked
)

func NewLink(text, target string, linkType LinkType) *Link { return &Link{linkType, text, target} }
func (n *Link) Clone() Elem {
	if n == nil {
		return nil
	}
	return &Link{
		n.linkType, n.text, n.target,
	}
}
func (n *Link) IsNil() bool { return n == nil }
func (n *Link) Literal() string {
	switch n.linkType {
	case LinkNaked:
		return n.target
	case LinkPlain:
		return fmt.Sprintf("<%s>", n.target)
	case LinkMedia:
		return fmt.Sprintf("![%s](%s)", n.text, n.target)
	default:
		return fmt.Sprintf("[%s](%s)", n.text, n.target)
	}
}
func (n *Link) FormatText() string { return n.Literal() }
func (n *Link) Markdown() string   { return n.Literal() }

func (n *Link) String() string {
	return n.Literal()
}
func (n *Link) IsMedia() bool  { return n.linkType == LinkMedia }
func (n *Link) Text() string   { return n.text }
func (n *Link) Target() string { return n.target }

type Code struct {
	codeType CodeType
	lit      string
}

type CodeType int

const (
	CodeInline CodeType = iota + 1
	CodeBlock
)

var _ Elem = (*Code)(nil)

func NewCode(text string, codeType CodeType) *Code { return &Code{codeType, text} }
func (n *Code) Clone() Elem {
	if n == nil {
		return nil
	}
	return &Code{
		n.codeType, n.lit,
	}
}
func (n *Code) IsNil() bool { return n == nil }
func (n *Code) Literal() string {
	if n.codeType == CodeBlock {
		return fmt.Sprintf("```%s```", n.lit)
	}
	return fmt.Sprintf("`%s`", n.lit)
}
func (n *Code) FormatText() string { return n.Literal() }
func (n *Code) Markdown() string   { return n.Literal() }

// String replaces line separator with newlines
func (n *Code) String() string {
	return strings.ReplaceAll(n.Literal(), "\u2028", "\n")
}

type Twt struct {
	dt       *DateTime
	msg      []Elem
	mentions []*Mention
	tags     []*Tag
	links    []*Link
	hash     string
	subject  *Subject
	twter    types.Twter
	pos      int
}

var _ Line = (*Twt)(nil)
var _ types.Twt = (*Twt)(nil)

func NewTwt(twter types.Twter, dt *DateTime, elems ...Elem) *Twt {
	twt := &Twt{twter: twter, dt: dt, msg: make([]Elem, 0, len(elems))}

	for _, elem := range elems {
		twt.append(elem)
	}

	return twt
}
func MakeTwt(twter types.Twter, ts time.Time, text string) types.Twt {
	dt := NewDateTime(ts)
	twt := NewTwt(twter, dt, nil)
	twt.twter = twter

	r := strings.NewReader(" " + text)
	lexer := NewLexer(r)
	lexer.NextTok() // remove first token we added to avoid parsing as comment.
	parser := NewParser(lexer)

	for elem := parser.ParseElem(); elem != nil; elem = parser.ParseElem() {
		parser.push()
		twt.append(elem)
	}

	return twt
}
func (twt *Twt) append(elem Elem) {
	if elem == nil || elem.IsNil() {
		return
	}

	twt.msg = append(twt.msg, elem)

	if subject, ok := elem.(*Subject); ok && twt.subject == nil {
		twt.subject = subject
		if subject.tag != nil {
			twt.tags = append(twt.tags, subject.tag)
		}
	}

	if tag, ok := elem.(*Tag); ok {
		twt.tags = append(twt.tags, tag)
	}

	if mention, ok := elem.(*Mention); ok {
		twt.mentions = append(twt.mentions, mention)
	}

	if link, ok := elem.(*Link); ok {
		twt.links = append(twt.links, link)
	}
}
func (twt *Twt) IsNil() bool  { return twt == nil }
func (twt *Twt) FilePos() int { return twt.pos }
func (twt *Twt) IsZero() bool { return twt.IsNil() || twt.Literal() == "" || twt.Created().IsZero() }
func (twt *Twt) Literal() string {
	var b strings.Builder
	b.WriteString(twt.dt.Literal())
	b.WriteRune('\t')
	for _, s := range twt.msg {
		if s == nil || s.IsNil() {
			continue
		}
		b.WriteString(s.Literal())
	}
	b.WriteRune('\n')
	return b.String()
}
func (twt Twt) Clone() types.Twt {
	return twt.CloneTwt()
}
func (twt Twt) CloneTwt() *Twt {
	msg := make([]Elem, len(twt.msg))
	for i := range twt.msg {
		msg[i] = twt.msg[i].Clone()
	}
	return NewTwt(twt.twter, twt.dt, msg...)
}
func (twt *Twt) Text() string {
	var b strings.Builder
	for _, s := range twt.msg {
		b.WriteString(s.FormatText())
	}
	return b.String()
}
func (twt *Twt) GobEncode() ([]byte, error) {
	twter := twt.Twter()
	s := fmt.Sprintf(
		"%s\t%s\t%s\t%s\t%s\n",
		twter.Nick,
		twter.URL,
		twt.Hash(),
		twt.Created().Format(time.RFC3339),
		twt.Text(),
	)
	return []byte(s), nil
}
func (twt *Twt) GobDecode(data []byte) error {
	sp := strings.SplitN(string(data), "\t", 4)
	if len(sp) != 4 {
		return fmt.Errorf("unable to decode twt: %s ", data)
	}
	twter := types.Twter{Nick: sp[0], URL: sp[1]}
	t, err := ParseLine(sp[3], twter)
	if err != nil {
		return err
	}

	twt.hash = sp[3]
	if t, ok := t.(*Twt); ok {
		twt.dt = t.dt
		twt.msg = t.msg
		twt.mentions = t.mentions
		twt.tags = t.tags
		twt.links = t.links
		twt.subject = t.subject
		twt.twter = t.twter
	}

	return nil
}
func (twt Twt) MarshalJSON() ([]byte, error) {
	var tags types.TagList = twt.Tags()
	return json.Marshal(struct {
		Twter        types.Twter `json:"twter"`
		Text         string      `json:"text"`
		Created      time.Time   `json:"created"`
		MarkdownText string      `json:"markdownText"`

		// Dynamic Fields
		Hash    string   `json:"hash"`
		Tags    []string `json:"tags"`
		Subject string   `json:"subject"`
	}{
		Twter:        twt.Twter(),
		Text:         twt.Text(),
		Created:      twt.Created(),
		MarkdownText: twt.FormatText(types.MarkdownFmt, nil),

		// Dynamic Fields
		Hash:    twt.Hash(),
		Tags:    tags.Tags(),
		Subject: twt.Subject().Text(),
	})
}
func DecodeJSON(data []byte) (types.Twt, error) {
	enc := struct {
		Twter   types.Twter `json:"twter"`
		Text    string      `json:"text"`
		Created time.Time   `json:"created"`
		Hash    string      `json:"hash"`
	}{}
	err := json.Unmarshal(data, &enc)
	if err != nil {
		return types.NilTwt, err
	}

	twter := enc.Twter
	// line := fmt.Sprintf("%s\t%s\n", enc.Created, enc.Text)
	// t, err := ParseLine(line, twter)
	t := MakeTwt(twter, enc.Created, enc.Text)
	if err != nil || t == nil {
		return types.NilTwt, err
	}

	twt := &Twt{}
	twt.hash = enc.Hash
	if t, ok := t.(*Twt); ok {
		twt.dt = t.dt
		twt.msg = t.msg
		twt.mentions = t.mentions
		twt.tags = t.tags
		twt.links = t.links
		twt.subject = t.subject
		twt.twter = t.twter

		return twt, nil
	}

	return types.NilTwt, err
}
func (twt Twt) FormatTwt() string {
	var b strings.Builder
	b.WriteString(twt.dt.String())
	b.WriteRune('\t')
	for _, s := range twt.msg {
		if s == nil || s.IsNil() {
			continue
		}
		b.WriteString(s.FormatText())
	}
	b.WriteRune('\n')
	return b.String()
}
func (twt Twt) FormatText(mode types.TwtTextFormat, opts types.FmtOpts) string {
	twt = *twt.CloneTwt()

	if opts != nil {
		for i := range twt.tags {
			switch mode {
			case types.TextFmt:
				twt.tags[i].target = ""
			}
		}

		for i := range twt.mentions {
			switch mode {
			case types.TextFmt:
				if twt.mentions[i].domain == "" &&
					opts.IsLocalURL(twt.mentions[i].target) &&
					strings.HasSuffix(twt.mentions[i].target, "/twtxt.txt") {
					twt.mentions[i].domain = opts.LocalURL().Hostname()
				}
				twt.mentions[i].target = ""
			case types.MarkdownFmt, types.HTMLFmt:
				if opts.IsLocalURL(twt.mentions[i].target) && strings.HasSuffix(twt.mentions[i].target, "/twtxt.txt") {
					twt.mentions[i].target = opts.UserURL(twt.mentions[i].target)
				} else {
					if twt.mentions[i].domain == "" {
						if u, err := url.Parse(twt.mentions[i].target); err == nil {
							twt.mentions[i].domain = u.Hostname()
						}
					}
					twt.mentions[i].target = opts.ExternalURL(twt.mentions[i].name, twt.mentions[i].target)
				}
			}
		}
	}

	switch mode {
	case types.TextFmt:
		var b strings.Builder
		for _, s := range twt.msg {
			b.WriteString(s.FormatText())
		}
		return b.String()
	case types.MarkdownFmt:
		var b strings.Builder
		for _, s := range twt.msg {
			b.WriteString(s.Markdown())
		}
		return b.String()
	case types.HTMLFmt:
		var b strings.Builder
		for _, s := range twt.msg {
			b.WriteString(s.Markdown())
		}
		return b.String()

	}
	return twt.Literal()
}
func (twt *Twt) ExpandLinks(opts types.FmtOpts, lookup types.FeedLookup) {
	for i, tag := range twt.tags {
		if tag.target == "" {
			tag.target = opts.URLForTag(tag.tag)
		}
		twt.tags[i] = tag
	}

	for i, m := range twt.mentions {
		if m.target == "" && lookup != nil {
			twter := lookup.FeedLookup(m.name)
			m.name = twter.Nick
			if sp := strings.SplitN(twter.Nick, "@", 2); len(sp) == 2 {
				m.name = sp[0]
				m.domain = sp[1]
			}
			m.target = twter.URL
		}

		fmt.Printf("Set %d - %v\n", i, m.target)
		twt.mentions[i] = m
	}
}
func (twt Twt) String() string     { return strings.ReplaceAll(twt.Literal(), "\u2028", "\n") }
func (twt Twt) Created() time.Time { return twt.dt.DateTime() }
func (twt Twt) Mentions() types.MentionList {
	lis := make([]types.TwtMention, len(twt.mentions))
	for i := range twt.mentions {
		lis[i] = twt.mentions[i]
	}
	return lis
}
func (twt Twt) Tags() types.TagList {
	lis := make([]types.TwtTag, len(twt.tags))
	for i := range twt.tags {
		lis[i] = twt.tags[i]
	}
	return lis
}
func (twt Twt) Links() types.LinkList {
	lis := make([]types.TwtLink, len(twt.links))
	for i := range twt.links {
		lis[i] = twt.links[i]
	}
	return lis
}
func (twt Twt) Twter() types.Twter { return twt.twter }
func (twt Twt) Hash() string {
	payload := fmt.Sprintf(
		"%s\n%s\n%s",
		twt.Twter().URL,
		twt.Created().Format(time.RFC3339),
		twt.FormatText(types.TextFmt, nil),
	)
	sum := blake2b.Sum256([]byte(payload))

	// Base32 is URL-safe, unlike Base64, and shorter than hex.
	encoding := base32.StdEncoding.WithPadding(base32.NoPadding)
	hash := strings.ToLower(encoding.EncodeToString(sum[:]))
	twt.hash = hash[len(hash)-types.TwtHashLength:]

	return twt.hash
}
func (twt Twt) Subject() types.Subject {
	if twt.subject == nil {
		twt.subject = NewSubjectTag(twt.Hash(), "")
	}
	return twt.subject
}

// Twts typedef to be able to attach sort methods
type Twts []*Twt

func (twts Twts) Len() int {
	return len(twts)
}
func (twts Twts) Less(i, j int) bool {
	if twts == nil {
		return false
	}

	return twts[i].Created().After(twts[j].Created())
}
func (twts Twts) Swap(i, j int) {
	twts[i], twts[j] = twts[j], twts[i]
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
	return MakeTwt(twter, ts, text)
}

func DefaultTwtManager() {
	types.SetTwtManager(&lextwtManager{})
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

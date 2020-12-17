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
//
// TWT     = DATE, tab, { MENTION | TAG | TEXT }, term;
// ```

import (
	"encoding/base32"
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

	f := &lextwtFile{}

	nLines, nErrors := 0, 0

	lexer := NewLexer(r)
	parser := NewParser(lexer)
	parser.SetTwter(twter)
	elem := parser.ParseLine()

	for elem != nil {
		nLines++

		switch e := elem.(type) {
		case *Comment:
			f.comments = append(f.comments, e)
		case *Twt:
			f.twts = append(f.twts, e)
		}

		elem = parser.ParseLine()
	}

	if (nLines+nErrors > 0) && nLines == nErrors {
		log.Warnf("erroneous feed dtected (nLines + nErrors > 0 && nLines == nErrors): %d/%d", nLines, nErrors)
		return nil, ErrParseElm
	}

	return f, nil
}
func ParseLine(line string, twter types.Twter) (twt types.Twt, err error) {
	r := strings.NewReader(line)
	lexer := NewLexer(r)
	parser := NewParser(lexer)
	parser.SetTwter(twter)

	return parser.ParseTwt(), parser.Errs()
}

// Lexer

type lexer struct {
	r io.Reader

	// simple ring buffer to xlate bytes to runes.
	rune rune
	last rune
	buf  []byte
	pos  int
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
	TokSPACE  TokType = "SPACE"  // White Space
	TokTAB    TokType = "TAB"    // Tab

	TokCOLON  TokType = ":"
	TokHYPHEN TokType = "-"
	TokDOT    TokType = "."
	TokPLUS   TokType = "+"
	TokT      TokType = "T"
	TokZ      TokType = "Z"

	TokHASH  TokType = "#"
	TokEQUAL TokType = "="

	TokAMP TokType = "@"
	TokLT  TokType = "<"
	TokGT  TokType = ">"
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

		default:
			l.loadString(" @#<>\u2028\n")
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
	if err != nil || size == 0 {
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
	nextTok Token

	twter types.Twter

	lit  []rune
	errs []error
}

type Token struct {
	Type    TokType
	Literal []rune
}

func NewParser(l *lexer) *parser {
	p := &parser{
		l: l,

		// as tokens are read they are appended here and stored in the resulting Elem.
		// the buffer is here so text can be recovered in the event a menton/tag fails to fully parse.
		// and to limit memory allocs.
		lit: make([]rune, 0, 512),
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
func (p *parser) ParseLine() Elem {
	var e Elem

	switch p.curTok.Type {
	case TokHASH:
		e = p.ParseComment()
	case TokNUMBER:
		e = p.ParseTwt()
	}
	if !(p.expect(TokNL) || p.expect(TokEOF)) {
		return nil
	}
	p.next()

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
	case TokHASH:
		e = p.ParseTag()
	case TokAMP:
		e = p.ParseMention()
	case TokNL, TokEOF:
		return nil
	default:
		e = p.ParseText(false)
	}

	// If parsing above failed convert to Text
	if e == nil || e.IsNil() {
		e = p.ParseText(true)
	}

	return e
}

// ParseComment from tokens
// Forms parsed:
//   # comment
//   # key = value
func (p *parser) ParseComment() *Comment {
	p.lit = p.lit[:0]
	if !p.expect(TokHASH) {
		return nil
	}

	p.lit = append(p.lit, p.curTok.Literal...)

	isKeyVal := false
	var label string
	var value []rune
	for !p.nextTokenIs(TokNL, TokEOF) {
		p.next()
		p.lit = append(p.lit, p.curTok.Literal...)

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

	return &Comment{string(p.lit), label, strings.TrimSpace(string(value))}
}

// ParseTwt from tokens
// Forms parsed:
//   [Date]\t... -> ParseElem (will consume all elems till end of line/file.)
func (p *parser) ParseTwt() *Twt {
	if !p.expect(TokNUMBER) {
		return nil
	}
	dt := p.ParseDateTime()

	if !p.expect(TokTAB) {
		return nil
	}
	p.next()
	var lis []Elem
	var mentions []*Mention
	var tags []*Tag
	elem := p.ParseElem()
	for elem != nil {
		lis = append(lis, elem)
		elem = p.ParseElem()

		// I could inline ParseElem here to avoid typecheck. But there doesn't appear to be a performance difference.
		if tag, ok := elem.(*Tag); ok {
			tags = append(tags, tag)
		}
		if mention, ok := elem.(*Mention); ok {
			mentions = append(mentions, mention)
		}
	}

	return &Twt{dt: dt, msg: lis, mentions: mentions, tags: tags, twter: p.twter}
}

// ParseDateTime from tokens
// Forms parsed:
//   YYYY-MM-DD'T'HH:mm[:ss[.nnnnnnnn]]('Z'|('+'|'-')th[:tm])
//   YYYY = year, MM = month, DD = day, HH = 24hour, mm = minute, ss = sec, nnnnnnnn = nsec, th = timezone hour, tm = timezone minute
func (p *parser) ParseDateTime() *DateTime {
	p.lit = p.lit[:0]

	var ok bool
	var year, month, day, hour, min, sec, nsec, sign, tzhour, tzmin int
	loc := time.UTC

	// Year
	p.lit = append(p.lit, p.curTok.Literal...)
	if year, ok = p.parseDigit(); !ok {
		return nil
	}

	// Hyphen
	p.lit = append(p.lit, p.curTok.Literal...)
	if !(p.expect(TokHYPHEN) && p.expectNext(TokNUMBER)) {
		return nil
	}

	// Month
	p.lit = append(p.lit, p.curTok.Literal...)
	if month, ok = p.parseDigit(); !ok {
		return nil
	}

	// Hyphen
	p.lit = append(p.lit, p.curTok.Literal...)
	if !(p.expect(TokHYPHEN) && p.expectNext(TokNUMBER)) {
		return nil
	}

	// Day
	p.lit = append(p.lit, p.curTok.Literal...)
	if day, ok = p.parseDigit(); !ok {
		return nil
	}

	// T
	p.lit = append(p.lit, p.curTok.Literal...)
	if !(p.expect(TokT) && p.expectNext(TokNUMBER)) {
		return nil
	}

	// Hour
	p.lit = append(p.lit, p.curTok.Literal...)
	if hour, ok = p.parseDigit(); !ok {
		return nil
	}

	// Colon
	p.lit = append(p.lit, p.curTok.Literal...)
	if !(p.expect(TokCOLON) && p.expectNext(TokNUMBER)) {
		return nil
	}

	// Minute
	p.lit = append(p.lit, p.curTok.Literal...)
	if min, ok = p.parseDigit(); !ok {
		return nil
	}

	// Optional Second
	if p.curTokenIs(TokCOLON) {
		p.lit = append(p.lit, p.curTok.Literal...)
		if !p.expectNext(TokNUMBER) {
			return nil
		}

		// Second
		p.lit = append(p.lit, p.curTok.Literal...)
		if sec, ok = p.parseDigit(); !ok {
			return nil
		}
	}

	// Optional NSec
	if p.curTokenIs(TokDOT) {
		p.lit = append(p.lit, p.curTok.Literal...)
		if !p.expectNext(TokNUMBER) {
			return nil
		}

		// NSecond
		p.lit = append(p.lit, p.curTok.Literal...)
		if nsec, ok = p.parseDigit(); !ok {
			return nil
		}
	}

	// UTC Timezone
	if p.curTokenIs(TokZ) {
		p.lit = append(p.lit, p.curTok.Literal...)
		p.next()

	} else if p.curTokenIs(TokPLUS) || p.curTokenIs(TokHYPHEN) {
		sign = 1
		tzfmt := "UTC+%02d%02d"

		p.lit = append(p.lit, p.curTok.Literal...)
		if p.curTokenIs(TokHYPHEN) {
			tzfmt = "UTC-%02d%02d"
			sign = -1
		}
		// TZHour
		if !p.expectNext(TokNUMBER) {
			return nil
		}
		p.lit = append(p.lit, p.curTok.Literal...)
		if tzhour, ok = p.parseDigit(); !ok {
			return nil
		}

		if tzhour > 24 {
			tzmin = tzhour % 100
			tzhour = tzhour / 100
		}

		// Optional tzmin with colon
		if p.curTokenIs(TokCOLON) {
			p.lit = append(p.lit, p.curTok.Literal...)
			if !p.expectNext(TokNUMBER) {
				return nil
			}

			// TZMin
			p.lit = append(p.lit, p.curTok.Literal...)
			if tzmin, ok = p.parseDigit(); !ok {
				return nil
			}
		}

		loc = time.FixedZone(fmt.Sprintf(tzfmt, tzhour, tzmin), sign*tzhour*3600+tzmin*60)
	}

	return &DateTime{dt: time.Date(year, time.Month(month), day, hour, min, sec, nsec, loc), lit: string(p.lit)}
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
	p.lit = p.lit[:0]
	var name, domain, target string

	p.lit = append(p.lit, p.curTok.Literal...)
	if p.curTokenIs(TokAMP) && p.nextTokenIs(TokSTRING) {
		p.lit = append(p.lit, p.curTok.Literal...)

		name = string(p.curTok.Literal)

		p.next()
		p.lit = append(p.lit, p.curTok.Literal...)

		if p.curTokenIs(TokAMP) && p.nextTokenIs(TokSTRING) {
			p.lit = append(p.lit, p.curTok.Literal...)

			domain = string(p.curTok.Literal)

			p.next()
			p.lit = append(p.lit, p.curTok.Literal...)
		}
	} else if p.curTokenIs(TokAMP) && p.nextTokenIs(TokLT) {
		p.lit = append(p.lit, p.curTok.Literal...)

		if !p.expectNext(TokSTRING) {
			return nil
		}
		p.lit = append(p.lit, p.curTok.Literal...)

		target = string(p.curTok.Literal)

		p.next()
		p.lit = append(p.lit, p.curTok.Literal...)

		if p.curTokenIs(TokAMP) && p.nextTokenIs(TokSTRING) {
			p.lit = append(p.lit, p.curTok.Literal...)

			name = target
			domain = string(p.curTok.Literal)

			p.next()
			p.lit = append(p.lit, p.curTok.Literal...)
		}

		if p.curTokenIs(TokSPACE) && p.nextTokenIs(TokSTRING) {
			p.lit = append(p.lit, p.curTok.Literal...)

			name = target
			target = string(p.curTok.Literal)

			p.next()
			p.lit = append(p.lit, p.curTok.Literal...)
		}

		if name == target {
			target = ""
		}

		if !p.expect(TokGT) {
			return nil
		}
		p.next()

	} else {
		return nil
	}

	return &Mention{lit: string(p.lit), name: name, domain: domain, target: target}
}

// ParseTag from tokens
// Forms parsed:
//   #tag
//   #<target>
//   #<tag target>
func (p *parser) ParseTag() *Tag {
	p.lit = p.lit[:0]
	var name, target string

	p.lit = append(p.lit, p.curTok.Literal...)
	if p.curTokenIs(TokHASH) && p.nextTokenIs(TokSTRING) {
		p.lit = append(p.lit, p.curTok.Literal...)

		name = string(p.curTok.Literal)

		p.next()
	} else if p.curTokenIs(TokHASH) && p.nextTokenIs(TokLT) {
		p.lit = append(p.lit, p.curTok.Literal...)

		if !p.expectNext(TokSTRING) {
			return nil
		}
		p.lit = append(p.lit, p.curTok.Literal...)

		target = string(p.curTok.Literal)

		p.next()
		p.lit = append(p.lit, p.curTok.Literal...)

		if p.curTokenIs(TokSPACE) && p.nextTokenIs(TokSTRING) {
			p.lit = append(p.lit, p.curTok.Literal...)

			name = target
			target = string(p.curTok.Literal)

			p.next()
			p.lit = append(p.lit, p.curTok.Literal...)
		}

		if name == target {
			target = ""
		}

		if !p.expect(TokGT) {
			return nil
		}
		p.next()

	} else {
		return nil
	}

	return &Tag{lit: string(p.lit), tag: name, target: target}
}

// ParseText from tokens.
// Forms parsed:
//   combination of string and space tokens.
func (p *parser) ParseText(keepbuf bool) *Text {
	if !keepbuf {
		p.lit = p.lit[:0]
	}

	p.lit = append(p.lit, p.curTok.Literal...)
	p.next()
	for p.curTokenIs(TokSTRING, TokSPACE) {
		p.lit = append(p.lit, p.curTok.Literal...)
		p.next()
	}

	return &Text{string(p.lit)}
}

func (p *parser) Errs() ListError {
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

// next promotes the next token and loads a new one.
// the parser keeps two buffers to store tokens and alternates them here.
func (p *parser) next() {
	p.curTok, p.nextTok = p.nextTok, p.curTok
	p.nextTok.Literal = p.nextTok.Literal[:0]
	p.l.NextTok()
	p.nextTok.Type = p.l.Token
	p.nextTok.Literal = append(p.nextTok.Literal, p.l.Literal...)
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
func (p *parser) expect(t TokType) bool {
	if p.curTokenIs(t) {
		return true
	}

	p.addError(fmt.Errorf("%w: expected current %v, found %v", ErrParseToken, t, p.curTok.Type))
	return false
}

// expectNext returns true if the current token matches and reads to next token. adds error if not.
func (p *parser) expectNext(t TokType) bool {
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
	IsNil() bool     // A typed nil will fail `elem == nil` We need to unbox to test.
	Literal() string // value as read from input.
	fmt.Stringer     // alias for Literal() for printing.
}

type Comment struct {
	comment string
	key     string
	value   string
}

var _ Elem = (*Comment)(nil)

func (n *Comment) IsNil() bool { return n == nil }
func (n *Comment) Literal() string {
	return n.comment + "\n"
}
func (n *Comment) String() string {
	return n.Literal()
}
func (n *Comment) KeyValue() (string, string) {
	return n.key, n.value
}

type Comments []*Comment

func (lis Comments) String() string {
	var b strings.Builder
	for _, line := range lis {
		b.WriteString(line.Literal())
	}
	return b.String()
}

func (lis Comments) Meta() *types.Meta {
	kv := make([]types.KeyPair, 0, len(lis))
	for _, c := range lis {
		if c.key != "" {
			kv = append(kv, types.KeyPair{c.key, c.value})
		}
	}

	return types.NewMeta(kv...)
}

type DateTime struct {
	lit string

	dt time.Time
}

var _ Elem = (*DateTime)(nil)

func (n *DateTime) IsNil() bool     { return n == nil }
func (n *DateTime) Literal() string { return n.lit }
func (n *DateTime) String() string  { return n.Literal() }
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
var _ types.Mention = (*Mention)(nil)

func (n *Mention) IsNil() bool        { return n == nil }
func (n *Mention) Twter() types.Twter { return types.Twter{Nick: n.name, URL: n.target} }
func (n *Mention) Literal() string    { return n.lit }
func (n *Mention) String() string     { return n.Literal() }
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

type Tag struct {
	lit string

	tag    string
	target string
	url    *url.URL
	err    error
}

var _ Elem = (*Tag)(nil)
var _ types.Tag = (*Tag)(nil)

func (n *Tag) IsNil() bool             { return n == nil }
func (n *Tag) Literal() string         { return n.lit }
func (n *Tag) String() string          { return n.Literal() }
func (n *Tag) Tag() string             { return n.tag }
func (n *Tag) Target() string          { return n.target }
func (n *Tag) SetTarget(target string) { n.target, n.url, n.err = target, nil, nil }
func (n *Tag) Err() error              { return n.err }
func (n *Tag) URL() *url.URL {
	if n.url == nil && n.err == nil {
		n.url, n.err = url.Parse(n.target)
	}
	return n.url
}

type Text struct {
	lit string
}

var _ Elem = (*Text)(nil)

func (n *Text) IsNil() bool { return n == nil }
func (n *Text) Literal() string {
	return n.lit
}
func (n *Text) String() string {
	return n.Literal()
}

type Twt struct {
	dt       *DateTime
	msg      []Elem
	mentions []*Mention
	tags     []*Tag
	hash     string
	twter    types.Twter
}

var _ Elem = (*Twt)(nil)
var _ types.Twt = (*Twt)(nil)

func (twt *Twt) IsNil() bool  { return twt == nil }
func (twt *Twt) IsZero() bool { return twt.IsNil() || twt.Literal() == "" || twt.Created().IsZero() }
func (twt *Twt) Literal() string {
	if twt == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(twt.dt.Literal())
	b.WriteRune('\t')
	for _, s := range twt.msg {
		b.WriteString(s.Literal())
	}
	b.WriteRune('\n')
	return b.String()
}
func (*Twt) Subject() string { return "" }
func (twt *Twt) Text() string {
	var b strings.Builder
	for _, s := range twt.msg {
		b.WriteString(s.Literal())
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
		return fmt.Errorf("unable to decode twt: %s", data)
	}
	twter := types.Twter{Nick: sp[0], URL: sp[1]}
	t, err := ParseLine(sp[4], twter)
	if err != nil {
		return err
	}

	twt.hash = sp[3]
	if t, ok := t.(*Twt); ok {
		twt.dt = t.dt
		twt.msg = t.msg
		twt.mentions = t.mentions
		twt.tags = t.tags
	}

	return nil
}
func DecodeJSON(data []byte) (types.Twt, error) {
	enc := struct {
		Twter   types.Twter `json:"twter"`
		Text    string      `json:"text"`
		Created string      `json:"created"`
		Hash    string      `json:"hash"`
	}{}
	err := json.Unmarshal(data, &enc)
	if err != nil {
		return types.NilTwt, err
	}

	twter := enc.Twter
	t, err := ParseLine(fmt.Sprintf("%s\t%s\n", enc.Created, enc.Text), twter)
	if err != nil {
		return types.NilTwt, err
	}
	twt := &Twt{}
	twt.hash = enc.Hash
	if t, ok := t.(*Twt); ok {
		twt.dt = t.dt
		twt.msg = t.msg
		twt.mentions = t.mentions
		twt.tags = t.tags
		return twt, nil
	}

	return types.NilTwt, err
}
func (twt *Twt) MarkdownText() string                                 { return twt.Literal() }
func (*Twt) SetFmtOpts(types.FmtOpts)                                 {}
func (twt *Twt) FormatText(types.TwtTextFormat, types.FmtOpts) string { return twt.Literal() }
func (twt *Twt) String() string                                       { return twt.Literal() }
func (twt *Twt) Created() time.Time                                   { return twt.dt.DateTime() }
func (twt *Twt) Mentions() types.MentionList {
	lis := make([]types.Mention, len(twt.mentions))
	for i := range twt.mentions {
		lis[i] = twt.mentions[i]
	}
	return lis
}
func (twt *Twt) Tags() types.TagList {
	lis := make([]types.Tag, len(twt.tags))
	for i := range twt.tags {
		lis[i] = twt.tags[i]
	}
	return lis
}
func (twt *Twt) Twter() types.Twter { return twt.twter }
func (twt *Twt) Hash() string {
	payload := twt.Twter().URL + "\n" + twt.Created().Format(time.RFC3339) + "\n" + twt.Literal()
	sum := blake2b.Sum256([]byte(payload))

	// Base32 is URL-safe, unlike Base64, and shorter than hex.
	encoding := base32.StdEncoding.WithPadding(base32.NoPadding)
	hash := strings.ToLower(encoding.EncodeToString(sum[:]))
	twt.hash = hash[len(hash)-types.TwtHashLength:]

	return twt.hash
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

func DefaultTwtManager() {
	types.SetTwtManager(&lextwtManager{})
}

type lextwtFile struct {
	twter    *types.Twter
	twts     types.Twts
	comments Comments
}

var _ types.TwtFile = (*lextwtFile)(nil)

func (r *lextwtFile) Twter() *types.Twter { return r.twter }
func (r *lextwtFile) Comment() string     { return r.comments.String() }
func (r *lextwtFile) Meta() *types.Meta   { return r.comments.Meta() }
func (r *lextwtFile) Twts() types.Twts    { return r.twts }

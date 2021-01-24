package lextwt

// EBNF
// lower = rune, Initial = []rune, CAPS = Element
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
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Token struct {
	Type    TokType
	Literal []rune
}

func (t Token) String() string {
	return fmt.Sprintf("%s[%s]", t.Type, string(t.Literal))
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

	TokAT     TokType = "@"
	TokLT     TokType = "<"
	TokGT     TokType = ">"
	TokLPAREN TokType = "("
	TokRPAREN TokType = ")"
	TokLBRACK TokType = "["
	TokRBRACK TokType = "]"
	TokBANG   TokType = "!"
	TokBSLASH TokType = `\`
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
// 	TokAT
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
			l.loadRune(TokAT)
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
			l.loadRune(TokBSLASH)
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
	var size int
	if !utf8.FullRune(l.buf[l.pos:l.size]) {
		copy(l.buf[:], l.buf[l.pos:l.size])
		l.pos = l.size - l.pos
		l.size = l.pos
		l.readBuf()
		l.pos = 0
	}
	if !utf8.FullRune(l.buf[l.pos:l.size]) {
		l.rune = EOF
		return
	}

	l.rune, size = utf8.DecodeRune(l.buf[l.pos:l.size])

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

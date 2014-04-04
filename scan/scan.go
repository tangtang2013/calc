package scan

import (
	"unicode"

	"github.com/rthornton128/calc1/token"
)

type Scanner struct {
	ch      rune
	offset  int
	roffset int
	src     string
	errors  ErrorList
}

func isDigit(r rune) bool {
	return unicode.IsDigit(r)
}

func (s *Scanner) Init(src string) {
  s.offset, s.roffset = 0, 0
  s.src = src
  s.errors = make(ErrorList, 16)

  s.next()
}

func (s *Scanner) Scan() (lit string, tok token.Token, pos token.Pos) {
	s.skipWhitespace()

	if isDigit(s.ch) {
		return s.scanNumber()
	}

	lit, pos = string(s.ch), token.Pos(s.offset)
	switch s.ch {
	case '(':
		tok = token.LPAREN
	case ')':
		tok = token.RPAREN
	case '+':
		tok = token.ADD
	case '-':
		tok = token.SUB
	case '*':
		tok = token.MUL
	case '/':
		tok = token.QUO
	case '%':
		tok = token.REM
	default:
		lit = ""
		if s.offset >= len(s.src)-1 {
			tok = token.EOF
		} else {
			tok = token.ILLEGAL
		}
	}

	s.next()

	return
}

func (s *Scanner) next() {
  s.ch = rune(0)
  if s.roffset < len(s.src) {
    s.offset = s.roffset
    s.ch = rune(s.src[s.offset])
    s.roffset++
  }
}

func (s *Scanner) scanNumber() (string, token.Token, token.Pos) {
	start := s.offset

	for isDigit(s.ch) {
		s.next()
	}

	return s.src[start:s.offset], token.INTEGER, token.Pos(start)
}

func (s *Scanner) skipWhitespace() {
	for unicode.IsSpace(s.ch) {
		s.next()
	}
}

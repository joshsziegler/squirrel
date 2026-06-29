package parser

import "strings"

// Lex turns a string containing SQL into Tokens. Unlike a naive whitespace splitter, it is
// quote-aware (so a "quoted", `quoted`, [quoted], or 'string' region is a single token regardless
// of internal spaces, commas, or parentheses) and comment-aware (a -- or /* */ comment is a single
// token). Whitespace, including newlines, is consumed and never emitted; a token records whether a
// newline preceded it via Token.NewlineBefore.
func Lex(s string) *Tokens {
	l := &lexer{src: s, line: 1}
	toks := make([]Token, 0)
	for {
		l.skipSpace()
		if l.peek() == 0 { // EOF
			break
		}
		start, line, nl := l.pos, l.line, l.nl
		c := l.peek()

		var t Token
		switch {
		case c == '-' && l.peekAt(1) == '-':
			t = Token{Type: Comment, Value: l.scanLineComment()}
		case c == '/' && l.peekAt(1) == '*':
			t = Token{Type: Comment, Value: l.scanBlockComment()}

		case c == '"':
			t = Token{Type: Ident, Quote: '"', Value: l.scanDelimited('"', true)}
		case c == '`':
			t = Token{Type: Ident, Quote: '`', Value: l.scanDelimited('`', true)}
		case c == '[':
			t = Token{Type: Ident, Quote: '[', Value: l.scanDelimited(']', false)}
		case c == '\'':
			t = Token{Type: String, Quote: '\'', Value: l.scanDelimited('\'', true)}

		case (c == 'x' || c == 'X') && l.peekAt(1) == '\'':
			l.advance() // consume the leading x/X
			t = Token{Type: Blob, Value: l.scanDelimited('\'', true)}
		case isDigit(c) || (c == '.' && isDigit(l.peekAt(1))):
			t = Token{Type: Number, Value: l.scanNumber()}

		case isIdentStart(c):
			// Keywords are NOT case-folded or specially classified yet: the SQLite keyword list
			// includes words used as identifiers (e.g. KEY, ACTION), so uppercasing them here would
			// corrupt column names. Bare words are emitted as Ident with their source text.
			t = Token{Type: Ident, Value: l.scanBareWord()}

		case isPunct(c):
			l.advance()
			t = Token{Type: Punct, Value: string(c)}

		default:
			t = Token{Type: Operator, Value: l.scanOperator()}
		}

		t.Pos, t.Line, t.NewlineBefore = start, line, nl
		toks = append(toks, t)
	}
	return NewTokens(toks)
}

// lexer holds the scanning state for a single Lex call.
type lexer struct {
	src  string
	pos  int
	line int
	nl   bool // whether a newline was seen while skipping whitespace before the current token
}

func (l *lexer) peek() byte {
	if l.pos < len(l.src) {
		return l.src[l.pos]
	}
	return 0
}

func (l *lexer) peekAt(n int) byte {
	if l.pos+n < len(l.src) {
		return l.src[l.pos+n]
	}
	return 0
}

func (l *lexer) advance() { l.pos++ }

// skipSpace consumes spaces, tabs, carriage returns, and newlines, tracking line numbers and
// whether any newline was seen (recorded into l.nl for the next token).
func (l *lexer) skipSpace() {
	l.nl = false
	for {
		switch l.peek() {
		case ' ', '\t', '\r':
			l.advance()
		case '\n':
			l.nl = true
			l.line++
			l.advance()
		default:
			return
		}
	}
}

// scanDelimited consumes an open ... close region and returns its UNQUOTED contents. When doubled
// is true, a doubled close character is an escaped literal (SQL's "", ”, and “ rules). Square
// brackets pass doubled=false: a ']' always closes (SQLite brackets cannot contain ']').
func (l *lexer) scanDelimited(close byte, doubled bool) string {
	l.advance() // consume the opening delimiter
	var b strings.Builder
	for {
		c := l.peek()
		if c == 0 { // unterminated; return what we have rather than looping forever
			break
		}
		if c == close {
			l.advance()
			if doubled && l.peek() == close { // escaped close, e.g. "" -> "
				b.WriteByte(close)
				l.advance()
				continue
			}
			break
		}
		if c == '\n' {
			l.line++
		}
		b.WriteByte(c)
		l.advance()
	}
	return b.String()
}

// scanLineComment consumes a -- comment to the end of the line (the newline is left for skipSpace)
// and returns the trimmed comment text.
func (l *lexer) scanLineComment() string {
	l.advance() // first '-'
	l.advance() // second '-'
	start := l.pos
	for l.peek() != '\n' && l.peek() != 0 {
		l.advance()
	}
	return strings.TrimSpace(l.src[start:l.pos])
}

// scanBlockComment consumes a /* ... */ comment and returns the trimmed inner text.
func (l *lexer) scanBlockComment() string {
	l.advance() // '/'
	l.advance() // '*'
	start := l.pos
	for {
		if l.peek() == 0 {
			break // unterminated
		}
		if l.peek() == '*' && l.peekAt(1) == '/' {
			text := l.src[start:l.pos]
			l.advance() // '*'
			l.advance() // '/'
			return strings.TrimSpace(text)
		}
		if l.peek() == '\n' {
			l.line++
		}
		l.advance()
	}
	return strings.TrimSpace(l.src[start:l.pos])
}

func (l *lexer) scanBareWord() string {
	start := l.pos
	for isIdentPart(l.peek()) {
		l.advance()
	}
	return l.src[start:l.pos]
}

func (l *lexer) scanNumber() string {
	start := l.pos
	if l.peek() == '0' && (l.peekAt(1) == 'x' || l.peekAt(1) == 'X') {
		l.advance()
		l.advance()
		for isHexDigit(l.peek()) {
			l.advance()
		}
		return l.src[start:l.pos]
	}
	for isDigit(l.peek()) {
		l.advance()
	}
	if l.peek() == '.' {
		l.advance()
		for isDigit(l.peek()) {
			l.advance()
		}
	}
	if l.peek() == 'e' || l.peek() == 'E' {
		l.advance()
		if l.peek() == '+' || l.peek() == '-' {
			l.advance()
		}
		for isDigit(l.peek()) {
			l.advance()
		}
	}
	return l.src[start:l.pos]
}

func (l *lexer) scanOperator() string {
	start := l.pos
	for isOperator(l.peek()) {
		l.advance()
	}
	if l.pos == start { // not a recognized operator char; consume one byte to guarantee progress
		l.advance()
	}
	return l.src[start:l.pos]
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }
func isAlpha(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }
func isHexDigit(c byte) bool {
	return isDigit(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

// isIdentStart reports whether c can start a bare identifier. Bytes >= 0x80 are treated as
// identifier characters so UTF-8 identifiers survive.
func isIdentStart(c byte) bool { return c == '_' || isAlpha(c) || c >= 0x80 }
func isIdentPart(c byte) bool  { return isIdentStart(c) || isDigit(c) || c == '$' }
func isPunct(c byte) bool      { return strings.IndexByte("(),;.", c) >= 0 }
func isOperator(c byte) bool   { return strings.IndexByte("=<>!|&+-*/%~", c) >= 0 }

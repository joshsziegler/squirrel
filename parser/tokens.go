package parser

import (
	"fmt"
	"strings"
)

func NewTokens(toks []Token) *Tokens {
	return &Tokens{toks: toks}
}

type Tokens struct {
	toks []Token
	i    int
}

// value returns the Value of the token at absolute index j, or "" if out of range.
func (t *Tokens) value(j int) string {
	if j >= 0 && j < len(t.toks) {
		return t.toks[j].Value
	}
	return ""
}

// Next token's value, or an empty string if there are no more tokens.
// This does not "take" the token, so further calls will return the same token.
func (t *Tokens) Next() string { return t.value(t.i) }

// NextType returns the type of the next token, or EOF if there are no more tokens.
func (t *Tokens) NextType() TokenType {
	if t.i < len(t.toks) {
		return t.toks[t.i].Type
	}
	return EOF
}

// NextNewlineBefore reports whether the next token was preceded by a newline. Used to tell an
// inline trailing comment from one on its own line.
func (t *Tokens) NextNewlineBefore() bool {
	if t.i < len(t.toks) {
		return t.toks[t.i].NewlineBefore
	}
	return false
}

// NextN returns the values of the next N tokens concatenated with single spaces.
// This does not "take" the tokens, so further calls will return the same tokens.
// If there are not n more tokens, returns the remainder (e.g. two tokens when three were requested).
func (t *Tokens) NextN(n int) string {
	s := make([]string, 0, n)
	for j := 0; j < n; j++ {
		if t.i+j >= len(t.toks) {
			break
		}
		s = append(s, t.toks[t.i+j].Value)
	}
	return strings.Join(s, " ")
}

// Peek at the value of the nth token from the current index, or "" if it does not exist.
// This does not "take" the token, so further calls will return the same token.
func (t *Tokens) Peek(n int) string { return t.value(t.i + n) }

// Take the next token (i.e. claim/consume it), returning its value.
// Does nothing if there are no more tokens.
func (t *Tokens) Take() string {
	if t.i < len(t.toks) {
		v := t.toks[t.i].Value
		t.i++
		return v
	}
	return ""
}

// TakeN takes the next N tokens (i.e. claim/consume them), clamping to the number remaining.
func (t *Tokens) TakeN(n int) {
	t.i += n
	if t.i > len(t.toks) {
		t.i = len(t.toks)
	}
}

// Return the last token (opposite of Take()).
func (t *Tokens) Return() {
	if t.i == 0 {
		fmt.Println("Error: Tried returning past zero")
		return
	}
	t.i -= 1
}

// ReturnN returns the last N tokens (opposite of TakeN()).
func (t *Tokens) ReturnN(n int) {
	if t.i-n < 0 {
		fmt.Println("Error: Tried returning past zero")
		return
	}
	t.i -= n
}

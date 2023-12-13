package main

func NewTokens(tokens []string) *Tokens {
	return &Tokens{
		i:      0,
		tokens: tokens,
		length: len(tokens),
	}
}

type Tokens struct {
	tokens []string
	i      int
	length int
}

// Next token or an empty string if there are no more tokens.
// This does not "take" the token, so further calls will return the same token.
func (t *Tokens) Next() string {
	if t.i < t.length {
		return t.tokens[t.i]
	}
	return ""
}

// NextN at the N next tokens concatenated together.
// If there are not n more tokens, returns the remainder (e.g. two tokens returned when three were requested).
func (t *Tokens) NextN(n int) string {
	s := ""
	for j := 0; j < n; j++ {
		if t.i+j >= t.length {
			return s
		}
		s += t.tokens[t.i+j]
	}
	return s
}

// Nth token from the current index or return an empty string if there are no more tokens.
func (t *Tokens) Nth(n int) string {
	j := t.i + n
	if j < t.length {
		return t.tokens[j]
	}
	return ""
}

// Take the next token (i.e. claim/consume it).
func (t *Tokens) Take() {
	t.i += 1 // TODO: What should I do if there are NOT n more tokens?
}

func (t *Tokens) TakeN(n int) {
	j := t.i + n
	if j < t.length {
		t.i += n
	}
	return // TODO: What should I do if there are NOT n more tokens?
}

type Table struct {
	SchemaName  string
	Name        string
	Temp        bool
	IfNotExists bool
	Columns     []Column
}

type Column struct {
	Name       string
	Type       string
	PrimaryKey bool
	Nullable   bool
	Unique     bool
}

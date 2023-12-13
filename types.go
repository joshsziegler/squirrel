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

// Next token or return empty string if there are no more tokens.
func (t *Tokens) Next() string {
	if t.i < t.length {
		return t.tokens[t.i]
	}
	return ""
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

// Peek at the N next tokens, or nil if there are not N more tokens.
func (t *Tokens) Peek(n int) []string {
	if t.i+n < t.length {
		return t.tokens[t.i : t.i+n]
	}
	return nil
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

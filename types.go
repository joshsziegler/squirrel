package main

type Tokens struct {
	tokens []string
	i      int
}

// Next token or return empty string if there are no more tokens.
func (t *Tokens) Next() string {
	if t.i < len(t.tokens) {
		return t.tokens[t.i]
	}
	return ""
}

// Take the next token (i.e. claim/consume it).
func (t *Tokens) Take() {
	t.i += 1
}

// Peek at the N next tokens, or nil if there are not N more tokens.
func (t *Tokens) Peek(n int) []string {
	if t.i+n < len(t.tokens) {
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

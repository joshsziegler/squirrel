package parser

import "strings"

// TokenType classifies a lexer Token. The parser largely works off Token.Value (via the Tokens
// accessors), but the type is needed to tell comments and string literals apart from identifiers.
type TokenType int

const (
	EOF      TokenType = iota
	Ident              // bare OR quoted identifier; Value is the UNQUOTED text
	String             // 'literal'; Value is the content with '' un-escaped
	Number             // 123, 1.5, 0xFF
	Blob               // x'AB12'
	Punct              // ( ) , ; .
	Operator           // = < > || + - * / ... (mostly consumed by expression skippers)
	Comment            // -- line  or  /* block */ ; Value is the trimmed comment text
)

// isKeyword reports whether this token is the given keyword: an UNQUOTED bare word matching
// case-insensitively. Quoted identifiers (e.g. "check" or [check]) are never keywords, per SQLite,
// which is what makes case-insensitive keyword matching safe against keyword-named columns.
func (tok Token) isKeyword(kw string) bool {
	return tok.Type == Ident && tok.Quote == 0 && strings.EqualFold(tok.Value, kw)
}

// Token is a single lexical unit produced by Lex.
type Token struct {
	Type  TokenType
	Value string // unquoted for Ident/String; raw text otherwise
	Quote byte   // 0 for bare; '"' '`' '[' for quoted idents; '\'' for strings
	Pos   int    // byte offset of the token start, for error messages
	Line  int    // 1-based source line, for error messages
	// NewlineBefore is true when the whitespace preceding this token contained a newline. It lets
	// the parser tell an inline trailing comment from a comment on its own line, now that newlines
	// are no longer emitted as tokens.
	NewlineBefore bool
}

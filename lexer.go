package main

import "strings"

// Lex turns a string containing SQL into tokens, leaving control characters and newlines.
func Lex(s string) *Tokens {
	tokens := []string{}
	lines := strings.Split(s, "\n")
	for _, line := range lines {
		token := ""
		for _, ch := range line {
			switch ch {
			case ' ', '	': // space or tab
				if token != "" {
					tokens = append(tokens, token)
					token = ""
				}
			case ',', '(', ')', ';':
				if token != "" {
					tokens = append(tokens, token)
					token = ""
				}
				tokens = append(tokens, string(ch))
			default:
				token += string(ch)
			}
		}
		if token != "" {
			tokens = append(tokens, token)
		}
		tokens = append(tokens, "\n") // Add explicit newline for line-dependent semantics, like comments
	}
	return NewTokens(tokens)
}

package main

import "strings"

// Lex turns a string containing SQL into tokens, leaving control characters and newlines.
func Lex(s string) *Tokens {
	tokens := []string{}
	lines := strings.Split(s, "\n")
	for _, line := range lines {
		token := ""
		for j := 0; j < len(line); j++ {
			ch := line[j]
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
			case '-': // Comment delimiter
				if len(line) > j+1 && line[j+1] == '-' {
					if token != "" { //  Append previous token if any
						tokens = append(tokens, token, "--")
						token = ""
					} else {
						tokens = append(tokens, "--")
					}
					j += 1 // Skip ahead to compensate for both characters
					continue
				}
				fallthrough // Not a comment delimiter
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

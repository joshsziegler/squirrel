package main

import (
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var (
	caser    = cases.Title(language.English)
	acronyms = map[string]string{
		"id":  "ID",
		"cpu": "CPU",
		"gpu": "GPU",
		"aws": "AWS",
		"ssl": "SSL",
		"url": "URL",
		"ip":  "IP",
		"pid": "PID",
	}
)

// ToGoName takes a snake_case name and converts it to CamelCase per Go conventions.
func ToGoName(s string) string {
	s = strings.ToLower(s) // Convert to lowercase to match against acronyms
	words := strings.Split(s, "_")
	name := ""
	for _, word := range words {
		acronym, found := acronyms[word]
		if found { // If this was an acronym, use the case provided.
			name += acronym
		} else { // Otherwise, use English title case rules
			name += caser.String(word)
		}
	}
	return name
}

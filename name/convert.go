package name

import (
	"strings"

	"github.com/gertd/go-pluralize"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var (
	pluralizer = pluralize.NewClient()
	caser      = cases.Title(language.English)
	acronyms   = map[string]string{
		"id":  "ID",
		"cpu": "CPU",
		"gpu": "GPU",
		"aws": "AWS",
		"ssl": "SSL",
		"url": "URL",
		"ip":  "IP",
		"pid": "PID",
		"uid": "UID",
		"gid": "GID",
		"os":  "OS",
	}
)

// ToGo converts a snake_case name to CamelCase -- per Go conventions -- and singularizes it.
func ToGo(s string) string {
	s = strings.ToLower(s) // Convert to lowercase to match against acronyms
	words := strings.Split(s, "_")
	name := ""
	for _, word := range words {
		acronym, found := acronyms[word]
		if found { // If this was an acronym, use the case provided.
			name += acronym
		} else { // Otherwise, use English title case rules and change to singular
			word = Singular(word) // TODO: Does this belong here, and should it only be for the last word (e.g. PluginSupportsSystem)?
			name += caser.String(word)
		}
	}
	return name
}

func Singular(s string) string {
	return pluralizer.Singular(s)
}

func Plural(s string) string {
	return pluralizer.Plural(s)
}

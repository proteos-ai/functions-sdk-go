//go:build goexperiment.jsonv2

package fn

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// jsonTagName returns the JSON name a `json` struct tag gives its field, or ""
// when the tag names nothing usable and encoding/json falls back to the Go
// field name.
//
// It mirrors parseFieldOptions in encoding/json/v2, which backs encoding/json
// when the jsonv2 experiment is on (the default from Go 1.27). This build
// constraint selects it exactly as the standard library does;
// declared-tag-name.go mirrors the original implementation.
func jsonTagName(tag string) string {
	if tag == "" || strings.HasPrefix(tag, ",") {
		return ""
	}

	// Almost any name is accepted up to the first comma, backslash or quote.
	rest := strings.TrimLeftFunc(tag, func(char rune) bool {
		return !strings.ContainsRune(",\\'\"`", char)
	})
	name := tag[:len(tag)-len(rest)]

	// A name that stops at a backslash or quote is malformed. What survives is
	// its leading Go identifier; without one the tag names nothing.
	if rest != "" && !strings.HasPrefix(rest, ",") {
		first, _ := utf8.DecodeRuneInString(tag)
		if first != '_' && !unicode.IsLetter(first) {
			return ""
		}
		name = tag[:len(tag)-len(strings.TrimLeftFunc(tag, isIdentifierRune))]
	}

	if !utf8.ValidString(name) {
		name = string([]rune(name)) // invalid UTF-8 becomes utf8.RuneError
	}
	return name
}

func isIdentifierRune(char rune) bool {
	return char == '_' || unicode.IsLetter(char) || unicode.IsNumber(char)
}

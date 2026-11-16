//go:build !goexperiment.jsonv2

package fn

import (
	"strings"
	"unicode"
)

// jsonTagName returns the JSON name a `json` struct tag gives its field, or ""
// when the tag names nothing usable and encoding/json falls back to the Go
// field name.
//
// It mirrors the original encoding/json, which this build constraint selects
// exactly as the standard library does. declared-tag-name-jsonv2.go mirrors
// the implementation Go 1.27 switched to by default; the two disagree on
// malformed names.
func jsonTagName(tag string) string {
	name, _, _ := strings.Cut(tag, ",")
	if !isValidTag(name) {
		return ""
	}
	return name
}

// isValidTag mirrors encoding/json's rule for a usable tag name: non-empty,
// and made only of letters, digits, and the punctuation json permits.
func isValidTag(name string) bool {
	if name == "" {
		return false
	}
	for _, char := range name {
		switch {
		case strings.ContainsRune("!#$%&()*+-./:;<=>?@[]^_{|}~ ", char):
			// Backslash and quote are reserved; other punctuation is allowed.
		case !unicode.IsLetter(char) && !unicode.IsDigit(char):
			return false
		}
	}
	return true
}

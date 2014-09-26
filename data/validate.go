package data

import (
	"unicode"
	"unicode/utf8"
)

// Validate only if all letters and digits
func ValidateName(name string) bool {
	if utf8.RuneCountInString(name) == 0 {
		return false
	}

	for _, r := range name {
		if !unicode.IsDigit(r) && !unicode.IsLetter(r) {
			return false
		}
	}

	return true
}

package content

import (
	"path"
	"strings"
	"unicode"
	"unicode/utf8"
)

const MaxVisibleTextBytes = 255

// CleanVisibleText applies the shared album-name, image-title, and attachment
// display-name policy. Existing stored values are not passed through this
// function when read; it is a mutation boundary only.
func CleanVisibleText(value string, allowEmpty bool) (string, error) {
	if !utf8.ValidString(value) {
		return "", validationError("visible_text_invalid_utf8", nil)
	}
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, value)
	value = path.Base(strings.ReplaceAll(value, "\\", "/"))
	if value == "." || value == ".." {
		value = ""
	}
	value = strings.TrimSpace(value)
	if value == "" && !allowEmpty {
		return "", validationError("visible_text_required", nil)
	}
	if len(value) > MaxVisibleTextBytes {
		return "", validationError("visible_text_too_long", nil)
	}
	return value, nil
}

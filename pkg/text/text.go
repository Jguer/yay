package text

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/leonelquinteros/gotext"
)

const (
	yDefault = "y"
	nDefault = "n"
)

// SplitDBFromName split apart db/package to db and package.
func SplitDBFromName(pkg string) (db, name string) {
	split := strings.SplitN(pkg, "/", 2)

	if len(split) == 2 {
		return split[0], split[1]
	}

	return "", split[0]
}

// LessRunes compares two rune values, and returns true if the first argument is lexicographicaly smaller.
func LessRunes(iRunes, jRunes []rune) bool {
	max := len(iRunes)
	if max > len(jRunes) {
		max = len(jRunes)
	}

	for idx := 0; idx < max; idx++ {
		ir := iRunes[idx]
		jr := jRunes[idx]

		lir := unicode.ToLower(ir)
		ljr := unicode.ToLower(jr)

		if lir != ljr {
			return lir < ljr
		}

		// the lowercase runes are the same, so compare the original
		if ir != jr {
			return ir < jr
		}
	}

	return len(iRunes) < len(jRunes)
}

// ContinueTask prompts if user wants to continue task.
// If NoConfirm is set the action will continue without user input.
func ContinueTask(s string, preset, noConfirm bool) bool {
	if noConfirm {
		return preset
	}

	var (
		response string
		postFix  string
		n        string
		y        string
		yes      = gotext.Get("yes")
		no       = gotext.Get("no")
	)

	// Only use localized "y" and "n" if they are latin characters.
	if nRune, _ := utf8.DecodeRuneInString(no); unicode.Is(unicode.Latin, nRune) {
		n = string(nRune)
	} else {
		n = nDefault
	}

	if yRune, _ := utf8.DecodeRuneInString(yes); unicode.Is(unicode.Latin, yRune) {
		y = string(yRune)
	} else {
		y = yDefault
	}

	if preset { // If default behavior is true, use y as default.
		postFix = fmt.Sprintf(" [%s/%s] ", strings.ToUpper(y), n)
	} else { // If default behavior is anything else, use n as default.
		postFix = fmt.Sprintf(" [%s/%s] ", y, strings.ToUpper(n))
	}

	OperationInfo(Bold(s), Bold(postFix))

	if _, err := fmt.Scanln(&response); err != nil {
		return preset
	}

	return strings.EqualFold(response, yes) ||
		strings.EqualFold(response, y) ||
		(!strings.EqualFold(yDefault, n) && strings.EqualFold(response, yDefault))
}

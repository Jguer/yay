package text

import (
	"strings"
	"unicode"
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

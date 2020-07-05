package main

import (
	"unicode"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v10/pkg/settings"
	"github.com/Jguer/yay/v10/pkg/text"
)

const gitEmptyTree = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

func stringSliceEqual(a, b []string) bool {
	if a == nil && b == nil {
		return true
	}

	if a == nil || b == nil {
		return false
	}

	if len(a) != len(b) {
		return false
	}

	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

func removeInvalidTargets(targets []string) []string {
	filteredTargets := make([]string, 0)

	for _, target := range targets {
		db, _ := splitDBFromName(target)

		if db == "aur" && config.Runtime.Mode == settings.ModeRepo {
			text.Warnln(gotext.Get("%s: can't use target with option --repo -- skipping", cyan(target)))
			continue
		}

		if db != "aur" && db != "" && config.Runtime.Mode == settings.ModeAUR {
			text.Warnln(gotext.Get("%s: can't use target with option --aur -- skipping", cyan(target)))
			continue
		}

		filteredTargets = append(filteredTargets, target)
	}

	return filteredTargets
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

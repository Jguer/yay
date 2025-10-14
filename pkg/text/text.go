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
	maxLen := min(len(iRunes), len(jRunes))

	for idx := 0; idx < maxLen; idx++ {
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

var RepoUrls = map[string]string{
	"core":             "https://archlinux.org/packages/core",
	"core-testing":     "https://archlinux.org/packages/core-testing",
	"extra":            "https://archlinux.org/packages/extra",
	"extra-testing":    "https://archlinux.org/packages/extra-testing",
	"gnome-unstable":   "https://archlinux.org/packages/gnome-unstable",
	"kde-unstable":     "https://archlinux.org/packages/kde-unstable",
	"multilib":         "https://archlinux.org/packages/multilib",
	"multilib-testing": "https://archlinux.org/packages/multilib-testing",
	"testing":          "https://archlinux.org/packages/testing",
	"aur":              "https://aur.archlinux.org/packages",
	"devel":            "https://aur.archlinux.org/packages",
}

func CreateOSC8Link(url, text string) string {
	osc8Start := "\033]8;;" + url + "\033\\"
	osc8End := "\033]8;;\033\\"
	return osc8Start + text + osc8End
}

func CreateRepoLink(repo, arch, pkgName, text string) string {
	if !UseColor {
		return text
	}

	urlBase, ok := RepoUrls[repo]
	if !ok {
		return text
	}

	var url string
	if repo == "aur" || repo == "devel" {
		url = urlBase + "/" + pkgName
	} else {
		url = urlBase + "/" + arch + "/" + pkgName
	}

	return CreateOSC8Link(url, text)
}

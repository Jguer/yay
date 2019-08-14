package upgrade

import (
	"unicode"

	"github.com/Jguer/yay/v10/pkg/text"
)

func getVersionDiff(oldVersion, newVersion string) (left, right string) {
	if oldVersion == newVersion {
		return oldVersion + text.Red(""), newVersion + text.Green("")
	}

	diffPosition := 0

	checkWords := func(str string, index int, words ...string) bool {
		for _, word := range words {
			wordLength := len(word)
			nextIndex := index + 1
			if (index < len(str)-wordLength) &&
				(str[nextIndex:(nextIndex+wordLength)] == word) {
				return true
			}
		}
		return false
	}

	for index, char := range oldVersion {
		charIsSpecial := !(unicode.IsLetter(char) || unicode.IsNumber(char))

		if (index >= len(newVersion)) || (char != rune(newVersion[index])) {
			if charIsSpecial {
				diffPosition = index
			}
			break
		}

		if charIsSpecial ||
			(((index == len(oldVersion)-1) || (index == len(newVersion)-1)) &&
				((len(oldVersion) != len(newVersion)) ||
					(oldVersion[index] == newVersion[index]))) ||
			checkWords(oldVersion, index, "rc", "pre", "alpha", "beta") {
			diffPosition = index + 1
		}
	}

	samePart := oldVersion[0:diffPosition]

	left = samePart + text.Red(oldVersion[diffPosition:])
	right = samePart + text.Green(newVersion[diffPosition:])

	return
}

package upgrade

import (
	"fmt"
	"unicode"

	"github.com/Jguer/yay/v11/pkg/db"
	"github.com/Jguer/yay/v11/pkg/intrange"
	"github.com/Jguer/yay/v11/pkg/text"
)

// Filter decides if specific package should be included in theincluded in the  results.
type Filter func(*Upgrade) bool

// Upgrade type describes a system upgrade.
type Upgrade = db.Upgrade

func StylizedNameWithRepository(u *Upgrade) string {
	return text.Bold(text.ColorHash(u.Repository)) + "/" + text.Bold(u.Name)
}

// upSlice is a slice of Upgrades.
type UpSlice struct {
	Up    []Upgrade
	Repos []string
}

func (u UpSlice) Len() int      { return len(u.Up) }
func (u UpSlice) Swap(i, j int) { u.Up[i], u.Up[j] = u.Up[j], u.Up[i] }

func (u UpSlice) Less(i, j int) bool {
	if u.Up[i].Repository == u.Up[j].Repository {
		iRunes := []rune(u.Up[i].Name)
		jRunes := []rune(u.Up[j].Name)

		return text.LessRunes(iRunes, jRunes)
	}

	for _, db := range u.Repos {
		if db == u.Up[i].Repository {
			return true
		} else if db == u.Up[j].Repository {
			return false
		}
	}

	iRunes := []rune(u.Up[i].Repository)
	jRunes := []rune(u.Up[j].Repository)

	return text.LessRunes(iRunes, jRunes)
}

func GetVersionDiff(oldVersion, newVersion string) (left, right string) {
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

	return left, right
}

// Print prints the details of the packages to upgrade.
func (u UpSlice) Print(logger *text.Logger) {
	longestName, longestVersion := 0, 0

	for k := range u.Up {
		upgrade := &u.Up[k]
		packNameLen := len(StylizedNameWithRepository(upgrade))
		packVersion, _ := GetVersionDiff(upgrade.LocalVersion, upgrade.RemoteVersion)
		packVersionLen := len(packVersion)
		longestName = intrange.Max(packNameLen, longestName)
		longestVersion = intrange.Max(packVersionLen, longestVersion)
	}

	namePadding := fmt.Sprintf("%%-%ds  ", longestName)
	versionPadding := fmt.Sprintf("%%-%ds", longestVersion)
	numberPadding := fmt.Sprintf("%%%dd  ", len(fmt.Sprintf("%v", len(u.Up))))

	for k := range u.Up {
		upgrade := &u.Up[k]
		left, right := GetVersionDiff(upgrade.LocalVersion, upgrade.RemoteVersion)

		logger.Printf(text.Magenta(fmt.Sprintf(numberPadding, len(u.Up)-k)))

		logger.Printf(namePadding, StylizedNameWithRepository(upgrade))

		logger.Printf("%s -> %s\n", fmt.Sprintf(versionPadding, left), right)
	}
}

package upgrade

import (
	"fmt"
	"unicode"

	"github.com/Jguer/yay/v10/pkg/intrange"
	"github.com/Jguer/yay/v10/pkg/text"
)

// Upgrade type describes a system upgrade.
type Upgrade struct {
	Name          string
	Repository    string
	LocalVersion  string
	RemoteVersion string
}

func (u *Upgrade) StylizedNameWithRepository() string {
	return text.Bold(text.ColorHash(u.Repository)) + "/" + text.Bold(u.Name)
}

// upSlice is a slice of Upgrades
type UpSlice []Upgrade

func (u UpSlice) Len() int      { return len(u) }
func (u UpSlice) Swap(i, j int) { u[i], u[j] = u[j], u[i] }

func (u UpSlice) Less(i, j int) bool {
	if u[i].Repository == u[j].Repository {
		iRunes := []rune(u[i].Name)
		jRunes := []rune(u[j].Name)
		return text.LessRunes(iRunes, jRunes)
	}

	iRunes := []rune(u[i].Repository)
	jRunes := []rune(u[j].Repository)
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
func (u UpSlice) Print() {
	longestName, longestVersion := 0, 0
	for _, pack := range u {
		packNameLen := len(pack.StylizedNameWithRepository())
		packVersion, _ := GetVersionDiff(pack.LocalVersion, pack.RemoteVersion)
		packVersionLen := len(packVersion)
		longestName = intrange.Max(packNameLen, longestName)
		longestVersion = intrange.Max(packVersionLen, longestVersion)
	}

	namePadding := fmt.Sprintf("%%-%ds  ", longestName)
	versionPadding := fmt.Sprintf("%%-%ds", longestVersion)
	numberPadding := fmt.Sprintf("%%%dd  ", len(fmt.Sprintf("%v", len(u))))

	for k, i := range u {
		left, right := GetVersionDiff(i.LocalVersion, i.RemoteVersion)

		fmt.Print(text.Magenta(fmt.Sprintf(numberPadding, len(u)-k)))

		fmt.Printf(namePadding, i.StylizedNameWithRepository())

		fmt.Printf("%s -> %s\n", fmt.Sprintf(versionPadding, left), right)
	}
}

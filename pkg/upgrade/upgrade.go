package upgrade

import (
	"fmt"
	"strings"

	"github.com/Jguer/yay/v12/pkg/db"
	"github.com/Jguer/yay/v12/pkg/intrange"
	"github.com/Jguer/yay/v12/pkg/query"
	"github.com/Jguer/yay/v12/pkg/text"
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

// Print prints the details of the packages to upgrade.
func (u UpSlice) Print(logger *text.Logger) {
	longestName, longestVersion := 0, 0

	for k := range u.Up {
		upgrade := &u.Up[k]
		packNameLen := len(StylizedNameWithRepository(upgrade))
		packVersion, _ := query.GetVersionDiff(upgrade.LocalVersion, upgrade.RemoteVersion)
		packVersionLen := len(packVersion)
		longestName = intrange.Max(packNameLen, longestName)
		longestVersion = intrange.Max(packVersionLen, longestVersion)
	}

	lenUp := len(u.Up)
	longestNumber := len(fmt.Sprintf("%v", lenUp))
	namePadding := fmt.Sprintf("%%-%ds  ", longestName)
	versionPadding := fmt.Sprintf("%%-%ds", longestVersion)
	numberPadding := fmt.Sprintf("%%%dd  ", longestNumber)

	for k := range u.Up {
		upgrade := &u.Up[k]
		left, right := query.GetVersionDiff(upgrade.LocalVersion, upgrade.RemoteVersion)

		logger.Printf(text.Magenta(fmt.Sprintf(numberPadding, lenUp-k)))

		logger.Printf(namePadding, StylizedNameWithRepository(upgrade))

		logger.Printf("%s -> %s\n", fmt.Sprintf(versionPadding, left), right)
		if upgrade.Extra != "" {
			logger.Println(strings.Repeat(" ", longestNumber), upgrade.Extra)
		}
	}
}

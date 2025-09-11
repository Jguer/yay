package upgrade

import (
	"fmt"
	"strings"

	"github.com/Jguer/yay/v12/pkg/db"
	"github.com/Jguer/yay/v12/pkg/query"
	"github.com/Jguer/yay/v12/pkg/text"
)

// Filter decides if specific package should be included in the results.
type Filter func(*Upgrade) bool

// Upgrade type describes a system upgrade.
type Upgrade = db.Upgrade

func StylizedNameWithRepository(u *Upgrade) string {
	return text.Bold(text.ColorHash(u.Repository)) + "/" + text.Bold(u.Name)
}

// upSlice is a slice of Upgrades.
type UpSlice struct {
	Up         []Upgrade
	Repos      []string
	PulledDeps []Upgrade
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
		switch db {
		case u.Up[i].Repository:
			return true
		case u.Up[j].Repository:
			return false
		}
	}

	iRunes := []rune(u.Up[i].Repository)
	jRunes := []rune(u.Up[j].Repository)

	return text.LessRunes(iRunes, jRunes)
}

// calculateFormatting calculates formatting parameters for printing upgrades
func calculateFormatting(upgrades []Upgrade) (longestName, longestVersion, longestNumber int) {
	for i := range upgrades {
		upgrade := &upgrades[i]
		packNameLen := len(StylizedNameWithRepository(upgrade))
		packVersion, _ := query.GetVersionDiff(upgrade.LocalVersion, upgrade.RemoteVersion)
		packVersionLen := len(packVersion)
		longestName = max(packNameLen, longestName)
		longestVersion = max(packVersionLen, longestVersion)
	}

	lenUp := len(upgrades)
	longestNumber = len(fmt.Sprintf("%v", lenUp))

	return
}

// Print prints the details of the packages to upgrade.
func (u UpSlice) Print(logger *text.Logger) {
	longestName, longestVersion, longestNumber := calculateFormatting(u.Up)

	namePadding := fmt.Sprintf("%%-%ds  ", longestName)
	versionPadding := fmt.Sprintf("%%-%ds", longestVersion)
	numberPadding := fmt.Sprintf("%%%dd  ", longestNumber)

	for k := range u.Up {
		upgrade := &u.Up[k]
		left, right := query.GetVersionDiff(upgrade.LocalVersion, upgrade.RemoteVersion)

		logger.Print(text.Magenta(fmt.Sprintf(numberPadding, len(u.Up)-k)))
		logger.Print(fmt.Sprintf(namePadding, StylizedNameWithRepository(upgrade)))
		logger.Printf("%s -> %s\n", fmt.Sprintf(versionPadding, left), right)
		if upgrade.Extra != "" {
			logger.Println(strings.Repeat(" ", longestNumber), upgrade.Extra)
		}
	}
}

func (u UpSlice) PrintDeps(logger *text.Logger) {
	longestName, longestVersion, longestNumber := calculateFormatting(u.PulledDeps)

	namePadding := fmt.Sprintf("  %s%%-%ds  ", strings.Repeat(" ", longestNumber), longestName)
	versionPadding := fmt.Sprintf("%%-%ds", longestVersion)

	for k := range u.PulledDeps {
		upgrade := &u.PulledDeps[k]
		left, right := query.GetVersionDiff(upgrade.LocalVersion, upgrade.RemoteVersion)

		logger.Printf("%s", fmt.Sprintf(namePadding, StylizedNameWithRepository(upgrade)))
		logger.Printf("%s -> %s\n", fmt.Sprintf(versionPadding, left), right)
		if upgrade.Extra != "" {
			logger.Println(strings.Repeat(" ", longestNumber), strings.ToLower(upgrade.Extra))
		}
	}

	logger.Println()
}

package upgrade

import (
	"fmt"
	"strings"

	"github.com/Jguer/yay/v13/pkg/db"
	"github.com/Jguer/yay/v13/pkg/query"
	"github.com/Jguer/yay/v13/pkg/text"
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

func (u UpSlice) compare(a, b Upgrade) int { //nolint:gocritic // slices.SortFunc comparator must take values; UpSlice.Up is []Upgrade
	if a.Repository == b.Repository {
		if text.LessRunes([]rune(a.Name), []rune(b.Name)) {
			return -1
		}
		if text.LessRunes([]rune(b.Name), []rune(a.Name)) {
			return 1
		}
		return 0
	}
	for _, db := range u.Repos {
		switch db {
		case a.Repository:
			return -1
		case b.Repository:
			return 1
		}
	}
	if text.LessRunes([]rune(a.Repository), []rune(b.Repository)) {
		return -1
	}
	if text.LessRunes([]rune(b.Repository), []rune(a.Repository)) {
		return 1
	}
	return 0
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
		ageTag := text.FormatAgeTag(upgrade.LastModified)
		if ageTag != "" {
			ageTag = " " + ageTag
		}
		logger.Printf("%s -> %s%s\n", fmt.Sprintf(versionPadding, left), right, ageTag)
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
		ageTag := text.FormatAgeTag(upgrade.LastModified)
		if ageTag != "" {
			ageTag = " " + ageTag
		}
		logger.Printf("%s -> %s%s\n", fmt.Sprintf(versionPadding, left), right, ageTag)
		if upgrade.Extra != "" {
			logger.Println(strings.Repeat(" ", longestNumber), strings.ToLower(upgrade.Extra))
		}
	}

	logger.Println()
}

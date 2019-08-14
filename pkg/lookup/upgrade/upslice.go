package upgrade

import (
	"fmt"

	"github.com/Jguer/go-alpm"
	"github.com/Jguer/yay/v10/pkg/text"
	"github.com/Jguer/yay/v10/pkg/types"
)

// Upgrade type describes a system upgrade.
type Upgrade struct {
	Name          string
	Repository    string
	LocalVersion  string
	RemoteVersion string
}

// UpSlice is a slice of Upgrades
type UpSlice []Upgrade

func (u UpSlice) Len() int      { return len(u) }
func (u UpSlice) Swap(i, j int) { u[i], u[j] = u[j], u[i] }

func (u UpSlice) Less(i, j int, alpmHandle *alpm.Handle) bool {
	if u[i].Repository == u[j].Repository {
		iRunes := []rune(u[i].Name)
		jRunes := []rune(u[j].Name)
		return types.LessRunes(iRunes, jRunes)
	}

	syncDB, err := alpmHandle.SyncDBs()
	if err != nil {
		iRunes := []rune(u[i].Repository)
		jRunes := []rune(u[j].Repository)
		return types.LessRunes(iRunes, jRunes)
	}

	less := false
	found := syncDB.ForEach(func(db alpm.DB) error {
		switch db.Name() {
		case u[i].Repository:
			less = true
		case u[j].Repository:
			less = false
		default:
			return nil
		}

		return fmt.Errorf("")
	})

	if found != nil {
		return less
	}

	iRunes := []rune(u[i].Repository)
	jRunes := []rune(u[j].Repository)
	return types.LessRunes(iRunes, jRunes)

}

// StylizedNameWithRepository returns a stilized string for printing
func (u Upgrade) StylizedNameWithRepository() string {
	return text.Bold(text.ColorHash(u.Repository)) + "/" + text.Bold(u.Name)
}

// Print prints the details of the packages to upgrade.
func (u UpSlice) Print() {
	longestName, longestVersion := 0, 0
	for _, pack := range u {
		packNameLen := len(pack.StylizedNameWithRepository())
		version, _ := getVersionDiff(pack.LocalVersion, pack.RemoteVersion)
		packVersionLen := len(version)
		longestName = types.Max(packNameLen, longestName)
		longestVersion = types.Max(packVersionLen, longestVersion)
	}

	namePadding := fmt.Sprintf("%%-%ds  ", longestName)
	versionPadding := fmt.Sprintf("%%-%ds", longestVersion)
	numberPadding := fmt.Sprintf("%%%dd  ", len(fmt.Sprintf("%v", len(u))))

	for k, i := range u {
		left, right := getVersionDiff(i.LocalVersion, i.RemoteVersion)

		fmt.Print(text.Magenta(fmt.Sprintf(numberPadding, len(u)-k)))

		fmt.Printf(namePadding, i.StylizedNameWithRepository())

		fmt.Printf("%s -> %s\n", fmt.Sprintf(versionPadding, left), right)
	}
}

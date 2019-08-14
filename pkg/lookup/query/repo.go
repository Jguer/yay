package query

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Jguer/go-alpm"
	"github.com/Jguer/yay/v10/pkg/runtime"
	"github.com/Jguer/yay/v10/pkg/text"
	"github.com/Jguer/yay/v10/pkg/types"
)

// Repo holds the results of a repository search.
type Repo []alpm.Package

const smallArrow = " ->"
const arrow = "==>"

// RepoSimple handles repo searches. Creates a Repo struct.
func RepoSimple(pkgInputN []string, alpmHandle *alpm.Handle, sortMode int) (s Repo, err error) {
	dbList, err := alpmHandle.SyncDBs()
	if err != nil {
		return
	}

	dbList.ForEach(func(db alpm.DB) error {
		if len(pkgInputN) == 0 {
			pkgs := db.PkgCache()
			s = append(s, pkgs.Slice()...)
		} else {
			pkgs := db.Search(pkgInputN)
			s = append(s, pkgs.Slice()...)
		}
		return nil
	})

	if sortMode == runtime.BottomUp {
		for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
			s[i], s[j] = s[j], s[i]
		}
	}

	return
}

// PrintSearch receives a RepoSearch type and outputs pretty text.
func (s Repo) PrintSearch(alpmHandle *alpm.Handle, searchMode int, sortMode int) {
	for i, res := range s {
		var toprint string
		if searchMode == runtime.NumberMenu {
			switch sortMode {
			case runtime.TopDown:
				toprint += text.Magenta(strconv.Itoa(i+1) + " ")
			case runtime.BottomUp:
				toprint += text.Magenta(strconv.Itoa(len(s)-i) + " ")
			default:
				fmt.Println("Invalid Sort Mode. Fix with yay -Y --bottomup --save")
			}
		} else if searchMode == runtime.Minimal {
			fmt.Println(res.Name())
			continue
		}

		toprint += text.Bold(text.ColorHash(res.DB().Name())) + "/" + text.Bold(res.Name()) +
			" " + text.Cyan(res.Version()) +
			text.Bold(" ("+text.Human(res.Size())+
				" "+text.Human(res.ISize())+") ")

		if len(res.Groups().Slice()) != 0 {
			toprint += fmt.Sprint(res.Groups().Slice(), " ")
		}

		localDB, err := alpmHandle.LocalDB()
		if err == nil {
			if pkg := localDB.Pkg(res.Name()); pkg != nil {
				if pkg.Version() != res.Version() {
					toprint += text.Bold(text.Green("(Installed: " + pkg.Version() + ")"))
				} else {
					toprint += text.Bold(text.Green("(Installed)"))
				}
			}
		}

		toprint += "\n    " + res.Description()
		fmt.Println(toprint)
	}
}

// FilterPackages filters packages based on source and type from local repository.
func FilterPackages(alpmHandle *alpm.Handle) (local []alpm.Package, remote []alpm.Package,
	localNames []string, remoteNames []string, err error) {
	localDB, err := alpmHandle.LocalDB()
	if err != nil {
		return
	}
	dbList, err := alpmHandle.SyncDBs()
	if err != nil {
		return
	}

	f := func(k alpm.Package) error {
		found := false
		// For each DB search for our secret package.
		_ = dbList.ForEach(func(d alpm.DB) error {
			if found {
				return nil
			}

			if d.Pkg(k.Name()) != nil {
				found = true
				local = append(local, k)
				localNames = append(localNames, k.Name())
			}
			return nil
		})

		if !found {
			remote = append(remote, k)
			remoteNames = append(remoteNames, k.Name())
		}
		return nil
	}

	err = localDB.PkgCache().ForEach(f)
	return
}

//SplitDBFromName splits apart db/package to db and package
func SplitDBFromName(pkg string) (string, string) {
	split := strings.SplitN(pkg, "/", 2)

	if len(split) == 2 {
		return split[0], split[1]
	}
	return "", split[0]
}

// PackageSlices separates an input slice into aur and repo slices
func PackageSlices(alpmHandle *alpm.Handle, mode types.TargetMode, toCheck []string) (aur []string, repo []string, err error) {
	dbList, err := alpmHandle.SyncDBs()
	if err != nil {
		return
	}

	for _, _pkg := range toCheck {
		db, name := SplitDBFromName(_pkg)
		found := false

		if db == "aur" || mode.IsAUR() {
			aur = append(aur, _pkg)
			continue
		} else if db != "" || mode.IsRepo() {
			repo = append(repo, _pkg)
			continue
		}

		_ = dbList.ForEach(func(db alpm.DB) error {
			if db.Pkg(name) != nil {
				found = true
				return fmt.Errorf("")

			}
			return nil
		})

		if !found {
			found = !dbList.FindGroupPkgs(name).Empty()
		}

		if found {
			repo = append(repo, _pkg)
		} else {
			aur = append(aur, _pkg)
		}
	}

	return
}

// RemoveInvalidTargets removes invalid targets from a targets slice.
func RemoveInvalidTargets(mode types.TargetMode, targets []string) []string {
	filteredTargets := make([]string, 0)

	for _, target := range targets {
		db, _ := SplitDBFromName(target)

		if db == "aur" && mode.IsRepo() {
			fmt.Printf("%s %s %s\n", text.Bold(text.Yellow(arrow)), text.Cyan(target), text.Bold("Can't use target with option --repo -- skipping"))
			continue
		}

		if db != "aur" && db != "" && mode.IsAUR() {
			fmt.Printf("%s %s %s\n", text.Bold(text.Yellow(arrow)), text.Cyan(target), text.Bold("Can't use target with option --aur -- skipping"))
			continue
		}

		filteredTargets = append(filteredTargets, target)
	}

	return filteredTargets
}

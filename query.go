package main

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	alpm "github.com/Jguer/go-alpm"
	"github.com/leonelquinteros/gotext"
	rpc "github.com/mikkeloscar/aur"

	"github.com/Jguer/yay/v10/pkg/query"
	"github.com/Jguer/yay/v10/pkg/settings"
	"github.com/Jguer/yay/v10/pkg/stringset"
	"github.com/Jguer/yay/v10/pkg/text"
)

// Query is a collection of Results
type aurQuery []rpc.Pkg

// Query holds the results of a repository search.
type repoQuery []alpm.Package

func (q aurQuery) Len() int {
	return len(q)
}

func (q aurQuery) Less(i, j int) bool {
	var result bool

	switch config.SortBy {
	case "votes":
		result = q[i].NumVotes > q[j].NumVotes
	case "popularity":
		result = q[i].Popularity > q[j].Popularity
	case "name":
		result = text.LessRunes([]rune(q[i].Name), []rune(q[j].Name))
	case "base":
		result = text.LessRunes([]rune(q[i].PackageBase), []rune(q[j].PackageBase))
	case "submitted":
		result = q[i].FirstSubmitted < q[j].FirstSubmitted
	case "modified":
		result = q[i].LastModified < q[j].LastModified
	case "id":
		result = q[i].ID < q[j].ID
	case "baseid":
		result = q[i].PackageBaseID < q[j].PackageBaseID
	}

	if config.SortMode == "bottomup" {
		return !result
	}

	return result
}

func (q aurQuery) Swap(i, j int) {
	q[i], q[j] = q[j], q[i]
}

func getSearchBy(value string) rpc.By {
	switch value {
	case "name":
		return rpc.Name
	case "maintainer":
		return rpc.Maintainer
	case "depends":
		return rpc.Depends
	case "makedepends":
		return rpc.MakeDepends
	case "optdepends":
		return rpc.OptDepends
	case "checkdepends":
		return rpc.CheckDepends
	default:
		return rpc.NameDesc
	}
}

// NarrowSearch searches AUR and narrows based on subarguments
func narrowSearch(pkgS []string, sortS bool) (aurQuery, error) {
	var r []rpc.Pkg
	var err error
	var usedIndex int

	by := getSearchBy(config.SearchBy)

	if len(pkgS) == 0 {
		return nil, nil
	}

	for i, word := range pkgS {
		r, err = rpc.SearchBy(word, by)
		if err == nil {
			usedIndex = i
			break
		}
	}

	if err != nil {
		return nil, err
	}

	if len(pkgS) == 1 {
		if sortS {
			sort.Sort(aurQuery(r))
		}
		return r, err
	}

	var aq aurQuery
	var n int

	for i := range r {
		match := true
		for j, pkgN := range pkgS {
			if usedIndex == j {
				continue
			}

			if !(strings.Contains(r[i].Name, pkgN) || strings.Contains(strings.ToLower(r[i].Description), pkgN)) {
				match = false
				break
			}
		}

		if match {
			n++
			aq = append(aq, r[i])
		}
	}

	if sortS {
		sort.Sort(aq)
	}

	return aq, err
}

// SyncSearch presents a query to the local repos and to the AUR.
func syncSearch(pkgS []string, alpmHandle *alpm.Handle) (err error) {
	pkgS = query.RemoveInvalidTargets(pkgS, config.Mode)
	var aurErr error
	var repoErr error
	var aq aurQuery
	var pq repoQuery

	if config.Mode == settings.ModeAUR || config.Mode == settings.ModeAny {
		aq, aurErr = narrowSearch(pkgS, true)
	}
	if config.Mode == settings.ModeRepo || config.Mode == settings.ModeAny {
		pq, repoErr = queryRepo(pkgS, alpmHandle)
		if repoErr != nil {
			return err
		}
	}

	switch config.SortMode {
	case "topdown":
		if config.Mode == settings.ModeRepo || config.Mode == settings.ModeAny {
			pq.printSearch(config.Alpm)
		}
		if config.Mode == settings.ModeAUR || config.Mode == settings.ModeAny {
			aq.printSearch(1, config.Alpm)
		}
	case "bottomup":
		if config.Mode == settings.ModeRepo || config.Mode == settings.ModeAny {
			aq.printSearch(1, config.Alpm)
		}
		if config.Mode == settings.ModeRepo || config.Mode == settings.ModeAny {
			pq.printSearch(config.Alpm)
		}
	default:
		return errors.New(gotext.Get("invalid sort mode. Fix with yay -Y --bottomup --save"))
	}

	if aurErr != nil {
		text.Errorln(gotext.Get("error during AUR search: %s", aurErr))
		text.Warnln(gotext.Get("Showing repo packages only"))
	}

	return nil
}

// SyncInfo serves as a pacman -Si for repo packages and AUR packages.
func syncInfo(pkgS []string, alpmHandle *alpm.Handle) error {
	var info []*rpc.Pkg
	missing := false
	pkgS = query.RemoveInvalidTargets(pkgS, config.Mode)
	aurS, repoS, err := packageSlices(pkgS, alpmHandle)
	if err != nil {
		return err
	}

	if len(aurS) != 0 {
		noDB := make([]string, 0, len(aurS))

		for _, pkg := range aurS {
			_, name := text.SplitDBFromName(pkg)
			noDB = append(noDB, name)
		}

		info, err = query.AURInfoPrint(noDB, config.RequestSplitN)
		if err != nil {
			missing = true
			fmt.Fprintln(os.Stderr, err)
		}
	}

	// Repo always goes first
	if len(repoS) != 0 {
		arguments := config.Flags()
		arguments.Targets = nil
		arguments.AddTarget(repoS...)
		err = show(passToPacman(arguments))

		if err != nil {
			return err
		}
	}

	if len(aurS) != len(info) {
		missing = true
	}

	if len(info) != 0 {
		for _, pkg := range info {
			PrintInfo(pkg, config.Info > 1)
		}
	}

	if missing {
		err = fmt.Errorf("")
	}

	return err
}

// Search handles repo searches. Creates a RepoSearch struct.
func queryRepo(pkgInputN []string, alpmHandle *alpm.Handle) (s repoQuery, err error) {
	dbList, err := alpmHandle.SyncDBs()
	if err != nil {
		return
	}

	_ = dbList.ForEach(func(db alpm.DB) error {
		if len(pkgInputN) == 0 {
			pkgs := db.PkgCache()
			s = append(s, pkgs.Slice()...)
		} else {
			pkgs := db.Search(pkgInputN)
			s = append(s, pkgs.Slice()...)
		}
		return nil
	})

	if config.SortMode == "bottomup" {
		for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
			s[i], s[j] = s[j], s[i]
		}
	}

	return
}

// PackageSlices separates an input slice into aur and repo slices
func packageSlices(toCheck []string, alpmHandle *alpm.Handle) (aur, repo []string, err error) {
	dbList, err := alpmHandle.SyncDBs()
	if err != nil {
		return nil, nil, err
	}

	for _, _pkg := range toCheck {
		db, name := text.SplitDBFromName(_pkg)
		found := false

		if db == "aur" || config.Mode == settings.ModeAUR {
			aur = append(aur, _pkg)
			continue
		} else if db != "" || config.Mode == settings.ModeRepo {
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

	return aur, repo, nil
}

// HangingPackages returns a list of packages installed as deps
// and unneeded by the system
// removeOptional decides whether optional dependencies are counted or not
func hangingPackages(removeOptional bool, alpmHandle *alpm.Handle) (hanging []string, err error) {
	localDB, err := alpmHandle.LocalDB()
	if err != nil {
		return
	}

	// safePackages represents every package in the system in one of 3 states
	// State = 0 - Remove package from the system
	// State = 1 - Keep package in the system; need to iterate over dependencies
	// State = 2 - Keep package and have iterated over dependencies
	safePackages := make(map[string]uint8)
	// provides stores a mapping from the provides name back to the original package name
	provides := make(stringset.MapStringSet)
	packages := localDB.PkgCache()

	// Mark explicit dependencies and enumerate the provides list
	setupResources := func(pkg alpm.Package) error {
		if pkg.Reason() == alpm.PkgReasonExplicit {
			safePackages[pkg.Name()] = 1
		} else {
			safePackages[pkg.Name()] = 0
		}

		_ = pkg.Provides().ForEach(func(dep alpm.Depend) error {
			provides.Add(dep.Name, pkg.Name())
			return nil
		})
		return nil
	}
	_ = packages.ForEach(setupResources)

	iterateAgain := true
	processDependencies := func(pkg alpm.Package) error {
		if state := safePackages[pkg.Name()]; state == 0 || state == 2 {
			return nil
		}

		safePackages[pkg.Name()] = 2

		// Update state for dependencies
		markDependencies := func(dep alpm.Depend) error {
			// Don't assume a dependency is installed
			state, ok := safePackages[dep.Name]
			if !ok {
				// Check if dep is a provides rather than actual package name
				if pset, ok2 := provides[dep.Name]; ok2 {
					for p := range pset {
						if safePackages[p] == 0 {
							iterateAgain = true
							safePackages[p] = 1
						}
					}
				}

				return nil
			}

			if state == 0 {
				iterateAgain = true
				safePackages[dep.Name] = 1
			}
			return nil
		}

		_ = pkg.Depends().ForEach(markDependencies)
		if !removeOptional {
			_ = pkg.OptionalDepends().ForEach(markDependencies)
		}
		return nil
	}

	for iterateAgain {
		iterateAgain = false
		_ = packages.ForEach(processDependencies)
	}

	// Build list of packages to be removed
	_ = packages.ForEach(func(pkg alpm.Package) error {
		if safePackages[pkg.Name()] == 0 {
			hanging = append(hanging, pkg.Name())
		}
		return nil
	})

	return hanging, err
}

// Statistics returns statistics about packages installed in system
func statistics(alpmHandle *alpm.Handle) (*struct {
	Totaln    int
	Expln     int
	TotalSize int64
}, error) {
	var tS int64 // TotalSize
	var nPkg int
	var ePkg int

	localDB, err := alpmHandle.LocalDB()
	if err != nil {
		return nil, err
	}

	for _, pkg := range localDB.PkgCache().Slice() {
		tS += pkg.ISize()
		nPkg++
		if pkg.Reason() == 0 {
			ePkg++
		}
	}

	info := &struct {
		Totaln    int
		Expln     int
		TotalSize int64
	}{
		nPkg, ePkg, tS,
	}

	return info, err
}

package main

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	alpm "github.com/Jguer/go-alpm/v2"
	"github.com/leonelquinteros/gotext"
	rpc "github.com/mikkeloscar/aur"

	"github.com/Jguer/yay/v10/pkg/db"
	"github.com/Jguer/yay/v10/pkg/query"
	"github.com/Jguer/yay/v10/pkg/settings"
	"github.com/Jguer/yay/v10/pkg/stringset"
	"github.com/Jguer/yay/v10/pkg/text"
)

// Query is a collection of Results
type aurQuery []rpc.Pkg

// Query holds the results of a repository search.
type repoQuery []alpm.IPackage

func (s repoQuery) Reverse() {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

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

	if config.SortMode == settings.BottomUp {
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
func syncSearch(pkgS []string, dbExecutor db.Executor) (err error) {
	pkgS = query.RemoveInvalidTargets(pkgS, config.Runtime.Mode)
	var aurErr error
	var aq aurQuery
	var pq repoQuery

	if config.Runtime.Mode == settings.ModeAUR || config.Runtime.Mode == settings.ModeAny {
		aq, aurErr = narrowSearch(pkgS, true)
	}
	if config.Runtime.Mode == settings.ModeRepo || config.Runtime.Mode == settings.ModeAny {
		pq = queryRepo(pkgS, dbExecutor)
	}

	switch config.SortMode {
	case settings.TopDown:
		if config.Runtime.Mode == settings.ModeRepo || config.Runtime.Mode == settings.ModeAny {
			pq.printSearch(dbExecutor)
		}
		if config.Runtime.Mode == settings.ModeAUR || config.Runtime.Mode == settings.ModeAny {
			aq.printSearch(1, dbExecutor)
		}
	case settings.BottomUp:
		if config.Runtime.Mode == settings.ModeAUR || config.Runtime.Mode == settings.ModeAny {
			aq.printSearch(1, dbExecutor)
		}
		if config.Runtime.Mode == settings.ModeRepo || config.Runtime.Mode == settings.ModeAny {
			pq.printSearch(dbExecutor)
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
func syncInfo(cmdArgs *settings.Arguments, pkgS []string, dbExecutor db.Executor) error {
	var info []*rpc.Pkg
	var err error
	missing := false
	pkgS = query.RemoveInvalidTargets(pkgS, config.Runtime.Mode)
	aurS, repoS := packageSlices(pkgS, dbExecutor)

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
		arguments := cmdArgs.Copy()
		arguments.ClearTargets()
		arguments.AddTarget(repoS...)
		err = config.Runtime.CmdRunner.Show(passToPacman(arguments))

		if err != nil {
			return err
		}
	}

	if len(aurS) != len(info) {
		missing = true
	}

	if len(info) != 0 {
		for _, pkg := range info {
			PrintInfo(pkg, cmdArgs.ExistsDouble("i"))
		}
	}

	if missing {
		err = fmt.Errorf("")
	}

	return err
}

// Search handles repo searches. Creates a RepoSearch struct.
func queryRepo(pkgInputN []string, dbExecutor db.Executor) repoQuery {
	s := repoQuery(dbExecutor.SyncPackages(pkgInputN...))

	if config.SortMode == settings.BottomUp {
		s.Reverse()
	}
	return s
}

// PackageSlices separates an input slice into aur and repo slices
func packageSlices(toCheck []string, dbExecutor db.Executor) (aur, repo []string) {
	for _, _pkg := range toCheck {
		dbName, name := text.SplitDBFromName(_pkg)

		if dbName == "aur" || config.Runtime.Mode == settings.ModeAUR {
			aur = append(aur, _pkg)
			continue
		} else if dbName != "" || config.Runtime.Mode == settings.ModeRepo {
			repo = append(repo, _pkg)
			continue
		}

		if dbExecutor.SyncSatisfierExists(name) ||
			len(dbExecutor.PackagesFromGroup(name)) != 0 {
			repo = append(repo, _pkg)
		} else {
			aur = append(aur, _pkg)
		}
	}

	return aur, repo
}

// HangingPackages returns a list of packages installed as deps
// and unneeded by the system
// removeOptional decides whether optional dependencies are counted or not
func hangingPackages(removeOptional bool, dbExecutor db.Executor) (hanging []string) {
	// safePackages represents every package in the system in one of 3 states
	// State = 0 - Remove package from the system
	// State = 1 - Keep package in the system; need to iterate over dependencies
	// State = 2 - Keep package and have iterated over dependencies
	safePackages := make(map[string]uint8)
	// provides stores a mapping from the provides name back to the original package name
	provides := make(stringset.MapStringSet)

	packages := dbExecutor.LocalPackages()
	// Mark explicit dependencies and enumerate the provides list
	for _, pkg := range packages {
		if pkg.Reason() == alpm.PkgReasonExplicit {
			safePackages[pkg.Name()] = 1
		} else {
			safePackages[pkg.Name()] = 0
		}

		for _, dep := range dbExecutor.PackageProvides(pkg) {
			provides.Add(dep.Name, pkg.Name())
		}
	}

	iterateAgain := true

	for iterateAgain {
		iterateAgain = false
		for _, pkg := range packages {
			if state := safePackages[pkg.Name()]; state == 0 || state == 2 {
				continue
			}

			safePackages[pkg.Name()] = 2
			deps := dbExecutor.PackageDepends(pkg)
			if !removeOptional {
				deps = append(deps, dbExecutor.PackageOptionalDepends(pkg)...)
			}

			// Update state for dependencies
			for _, dep := range deps {
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
					continue
				}

				if state == 0 {
					iterateAgain = true
					safePackages[dep.Name] = 1
				}
			}
		}
	}

	// Build list of packages to be removed
	for _, pkg := range packages {
		if safePackages[pkg.Name()] == 0 {
			hanging = append(hanging, pkg.Name())
		}
	}

	return hanging
}

// Statistics returns statistics about packages installed in system
func statistics(dbExecutor db.Executor) *struct {
	Totaln    int
	Expln     int
	TotalSize int64
} {
	var totalSize int64
	localPackages := dbExecutor.LocalPackages()
	totalInstalls := 0
	explicitInstalls := 0

	for _, pkg := range localPackages {
		totalSize += pkg.ISize()
		totalInstalls++
		if pkg.Reason() == alpm.PkgReasonExplicit {
			explicitInstalls++
		}
	}

	info := &struct {
		Totaln    int
		Expln     int
		TotalSize int64
	}{
		totalInstalls, explicitInstalls, totalSize,
	}

	return info
}

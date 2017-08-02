package main

import (
	"fmt"
	"sort"
	"strings"

	alpm "github.com/jguer/go-alpm"
	pac "github.com/jguer/yay/pacman"
	rpc "github.com/mikkeloscar/aur"
)

// Query is a collection of Results
type aurQuery []rpc.Pkg

// Query holds the results of a repository search.
type repoQuery []alpm.Package

func (q aurQuery) Len() int {
	return len(q)
}

func (q aurQuery) Less(i, j int) bool {
	if config.SortMode == BottomUp {
		return q[i].NumVotes < q[j].NumVotes
	}
	return q[i].NumVotes > q[j].NumVotes
}

func (q aurQuery) Swap(i, j int) {
	q[i], q[j] = q[j], q[i]
}

// MissingPackage warns if the Query was unable to find a package
func (q aurQuery) missingPackage(pkgS []string) {
	for _, depName := range pkgS {
		found := false
		for _, dep := range q {
			if dep.Name == depName {
				found = true
				break
			}
		}

		if !found {
			fmt.Println("\x1b[31mUnable to find", depName, "in AUR\x1b[0m")
		}
	}
	return
}

// NarrowSearch searches AUR and narrows based on subarguments
func narrowSearch(pkgS []string, sortS bool) (aurQuery, error) {
	if len(pkgS) == 0 {
		return nil, nil
	}

	r, err := rpc.Search(pkgS[0])
	if err != nil {
		return nil, err
	}

	if len(pkgS) == 1 {
		if sortS {
			sort.Sort(Query(r))
		}
		return r, err
	}

	var aq aurQuery
	var n int

	for _, res := range r {
		match := true
		for _, pkgN := range pkgS[1:] {
			if !(strings.Contains(res.Name, pkgN) || strings.Contains(strings.ToLower(res.Description), pkgN)) {
				match = false
				break
			}
		}

		if match {
			n++
			aq = append(aq, res)
		}
	}

	if sortS {
		sort.Sort(aq)
	}

	return aq, err
}

// SyncSearch presents a query to the local repos and to the AUR.
func syncSearch(pkgS []string) (err error) {
	aq, err := narrowSearch(pkgS, true)
	if err != nil {
		return err
	}
	pq, _, err := pac.Search(pkgS)
	if err != nil {
		return err
	}

	if config.SortMode == BottomUp {
		aq.printAURSearch(0)
		pq.PrintSearch()
	} else {
		pq.PrintSearch()
		aq.printAURSearch(0)
	}

	return nil
}

// SyncInfo serves as a pacman -Si for repo packages and AUR packages.
func syncInfo(pkgS []string, flags []string) (err error) {
	aurS, repoS, err := pac.PackageSlices(pkgS)
	if err != nil {
		return
	}

	if len(aurS) != 0 {
		q, err := rpc.Info(aurS)
		if err != nil {
			fmt.Println(err)
		}
		for _, aurP := range q {
			PrintInfo(&aurP)
		}
	}

	if len(repoS) != 0 {
		err = PassToPacman("-Si", repoS, flags)
	}

	return
}

// LocalStatistics returns installed packages statistics.
func localStatistics(version string) error {
	info, err := pac.Statistics()
	if err != nil {
		return err
	}

	foreignS, err := pac.ForeignPackages()
	if err != nil {
		return err
	}

	fmt.Printf("\n Yay version r%s\n", version)
	fmt.Println("\x1B[1;34m===========================================\x1B[0m")
	fmt.Printf("\x1B[1;32mTotal installed packages: \x1B[0;33m%d\x1B[0m\n", info.Totaln)
	fmt.Printf("\x1B[1;32mTotal foreign installed packages: \x1B[0;33m%d\x1B[0m\n", len(foreignS))
	fmt.Printf("\x1B[1;32mExplicitly installed packages: \x1B[0;33m%d\x1B[0m\n", info.Expln)
	fmt.Printf("\x1B[1;32mTotal Size occupied by packages: \x1B[0;33m%s\x1B[0m\n", Human(info.TotalSize))
	fmt.Println("\x1B[1;34m===========================================\x1B[0m")
	fmt.Println("\x1B[1;32mTen biggest packages\x1B[0m")
	pac.BiggestPackages()
	fmt.Println("\x1B[1;34m===========================================\x1B[0m")

	keys := make([]string, len(foreignS))
	i := 0
	for k := range foreignS {
		keys[i] = k
		i++
	}

	var q aurQuery
	var j int
	for i = len(keys); i != 0; i = j {
		j = i - config.RequestSplitN
		if j < 0 {
			j = 0
		}
		qtemp, err := rpc.Info(keys[j:i])
		q = append(q, qtemp...)
		if err != nil {
			return err
		}
	}

	var outcast []string
	for _, s := range keys {
		found := false
		for _, i := range q {
			if s == i.Name {
				found = true
				break
			}
		}
		if !found {
			outcast = append(outcast, s)
		}
	}

	if err != nil {
		return err
	}

	for _, res := range q {
		if res.Maintainer == "" {
			fmt.Printf("\x1b[1;31;40mWarning: \x1B[1;33;40m%s\x1b[0;37;40m is orphaned.\x1b[0m\n", res.Name)
		}
		if res.OutOfDate != 0 {
			fmt.Printf("\x1b[1;31;40mWarning: \x1B[1;33;40m%s\x1b[0;37;40m is out-of-date in AUR.\x1b[0m\n", res.Name)
		}
	}

	for _, res := range outcast {
		fmt.Printf("\x1b[1;31;40mWarning: \x1B[1;33;40m%s\x1b[0;37;40m is not available in AUR.\x1b[0m\n", res)
	}

	return nil
}

// Search handles repo searches. Creates a RepoSearch struct.
func queryRepo(pkgInputN []string) (s repoQuery, n int, err error) {
	dbList, err := AlpmHandle.SyncDbs()
	if err != nil {
		return
	}

	// BottomUp functions
	initL := func(len int) int {
		if config.SortMode == TopDown {
			return 0
		}
		return len - 1
	}
	compL := func(len int, i int) bool {
		if config.SortMode == TopDown {
			return i < len
		}
		return i > -1
	}
	finalL := func(i int) int {
		if config.SortMode == TopDown {
			return i + 1
		}
		return i - 1
	}

	dbS := dbList.Slice()
	lenDbs := len(dbS)
	for f := initL(lenDbs); compL(lenDbs, f); f = finalL(f) {
		pkgS := dbS[f].PkgCache().Slice()
		lenPkgs := len(pkgS)
		for i := initL(lenPkgs); compL(lenPkgs, i); i = finalL(i) {
			match := true
			for _, pkgN := range pkgInputN {
				if !(strings.Contains(pkgS[i].Name(), pkgN) || strings.Contains(strings.ToLower(pkgS[i].Description()), pkgN)) {
					match = false
					break
				}
			}

			if match {
				n++
				s = append(s, pkgS[i])
			}
		}
	}
	return
}

// PkgDependencies returns package dependencies not installed belonging to AUR
// 0 is Repo, 1 is Foreign.
func pkgDependencies(a *rpc.Pkg) (runDeps [2][]string, makeDeps [2][]string, err error) {
	var q aurQuery
	if len(a.Depends) == 0 && len(a.MakeDepends) == 0 {
		q, err = rpc.Info([]string{a.Name})
		if len(q) == 0 || err != nil {
			err = fmt.Errorf("Unable to search dependencies, %s", err)
			return
		}
	} else {
		q = append(q, *a)
	}

	depSearch := pacman.BuildDependencies(a.Depends)
	if len(a.Depends) != 0 {
		runDeps[0], runDeps[1] = depSearch(q[0].Depends, true, false)
		if len(runDeps[0]) != 0 || len(runDeps[1]) != 0 {
			fmt.Println("\x1b[1;32m=>\x1b[1;33m Run Dependencies: \x1b[0m")
			printDeps(runDeps[0], runDeps[1])
		}
	}

	if len(a.MakeDepends) != 0 {
		makeDeps[0], makeDeps[1] = depSearch(q[0].MakeDepends, false, false)
		if len(makeDeps[0]) != 0 || len(makeDeps[1]) != 0 {
			fmt.Println("\x1b[1;32m=>\x1b[1;33m Make Dependencies: \x1b[0m")
			printDeps(makeDeps[0], makeDeps[1])
		}
	}
	depSearch(a.MakeDepends, false, true)

	err = nil
	return
}

// PackageSlices separates an input slice into aur and repo slices
func packageSlices(toCheck []string) (aur []string, repo []string, err error) {
	dbList, err := AlpmHandle.SyncDbs()
	if err != nil {
		return
	}

	for _, pkg := range toCheck {
		found := false

		_ = dbList.ForEach(func(db alpm.Db) error {
			if found {
				return nil
			}

			_, err = db.PkgByName(pkg)
			if err == nil {
				found = true
				repo = append(repo, pkg)
			}
			return nil
		})

		if !found {
			if _, errdb := dbList.PkgCachebyGroup(pkg); errdb == nil {
				repo = append(repo, pkg)
			} else {
				aur = append(aur, pkg)
			}
		}
	}

	err = nil
	return
}

// ForeignPackages returns a map of foreign packages, with their version and date as values.
func foreignPackages() (foreign map[string]alpm.Package, err error) {
	localDb, err := AlpmHandle.LocalDb()
	if err != nil {
		return
	}
	dbList, err := AlpmHandle.SyncDbs()
	if err != nil {
		return
	}

	foreign = make(map[string]alpm.Package)

	f := func(k alpm.Package) error {
		found := false
		_ = dbList.ForEach(func(d alpm.Db) error {
			if found {
				return nil
			}
			_, err = d.PkgByName(k.Name())
			if err == nil {
				found = true
			}
			return nil
		})

		if !found {
			foreign[k.Name()] = k
		}
		return nil
	}

	err = localDb.PkgCache().ForEach(f)
	return
}

// HangingPackages returns a list of packages installed as deps
// and unneeded by the system
func hangingPackages() (hanging []string, err error) {
	localDb, err := AlpmHandle.LocalDb()
	if err != nil {
		return
	}

	f := func(pkg alpm.Package) error {
		if pkg.Reason() != alpm.PkgReasonDepend {
			return nil
		}
		requiredby := pkg.ComputeRequiredBy()
		if len(requiredby) == 0 {
			hanging = append(hanging, pkg.Name())
			fmt.Printf("%s: \x1B[0;33m%s\x1B[0m\n", pkg.Name(), Human(pkg.ISize()))

		}
		return nil
	}

	err = localDb.PkgCache().ForEach(f)
	return
}

// Statistics returns statistics about packages installed in system
func statistics() (info struct {
	Totaln    int
	Expln     int
	TotalSize int64
}, err error) {
	var tS int64 // TotalSize
	var nPkg int
	var ePkg int

	localDb, err := AlpmHandle.LocalDb()
	if err != nil {
		return
	}

	for _, pkg := range localDb.PkgCache().Slice() {
		tS += pkg.ISize()
		nPkg++
		if pkg.Reason() == 0 {
			ePkg++
		}
	}

	info = struct {
		Totaln    int
		Expln     int
		TotalSize int64
	}{
		nPkg, ePkg, tS,
	}

	return
}

// SliceHangingPackages returns a list of packages installed as deps
// and unneeded by the system from a provided list of package names.
func sliceHangingPackages(pkgS []string) (hanging []string) {
	localDb, err := AlpmHandle.LocalDb()
	if err != nil {
		return
	}

big:
	for _, pkgName := range pkgS {
		for _, hangN := range hanging {
			if hangN == pkgName {
				continue big
			}
		}

		pkg, err := localDb.PkgByName(pkgName)
		if err == nil {
			if pkg.Reason() != alpm.PkgReasonDepend {
				continue
			}

			requiredby := pkg.ComputeRequiredBy()
			if len(requiredby) == 0 {
				hanging = append(hanging, pkgName)
				fmt.Printf("%s: \x1B[0;33m%s\x1B[0m\n", pkg.Name(), Human(pkg.ISize()))
			}
		}
	}
	return
}

package main

import (
	"fmt"
	"sort"
	"strings"

	alpm "github.com/jguer/go-alpm"
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

// FilterPackages filters packages based on source and type from local repository.
func filterPackages() (local []alpm.Package, remote []alpm.Package,
	localNames []string, remoteNames []string, err error) {
	localDb, err := alpmHandle.LocalDb()
	if err != nil {
		return
	}
	dbList, err := alpmHandle.SyncDbs()
	if err != nil {
		return
	}

	f := func(k alpm.Package) error {
		found := false
		// For each DB search for our secret package.
		_ = dbList.ForEach(func(d alpm.Db) error {
			if found {
				return nil
			}
			_, err := d.PkgByName(k.Name())
			if err == nil {
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

	err = localDb.PkgCache().ForEach(f)
	return
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
			sort.Sort(aurQuery(r))
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
	pq, _, err := queryRepo(pkgS)
	if err != nil {
		return err
	}

	if config.SortMode == BottomUp {
		aq.printSearch(1)
		pq.printSearch()
	} else {
		pq.printSearch()
		aq.printSearch(1)
	}

	return nil
}

// SyncInfo serves as a pacman -Si for repo packages and AUR packages.
func syncInfo(pkgS []string, flags []string) (err error) {
	aurS, repoS, err := packageSlices(pkgS)
	if err != nil {
		return
	}

	//repo always goes first
	if len(repoS) != 0 {
		arguments := makeArguments()
		arguments.addArg("S", "i")
		//arguments.addArg(flags...)
		arguments.addTarget(repoS...)
		err = passToPacman(arguments)

		if err != nil {
			return
		}
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

	return
}

// Search handles repo searches. Creates a RepoSearch struct.
func queryRepo(pkgInputN []string) (s repoQuery, n int, err error) {
	dbList, err := alpmHandle.SyncDbs()
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

// PackageSlices separates an input slice into aur and repo slices
func packageSlices(toCheck []string) (aur []string, repo []string, err error) {
	dbList, err := alpmHandle.SyncDbs()
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

// HangingPackages returns a list of packages installed as deps
// and unneeded by the system
func hangingPackages() (hanging []string, err error) {
	localDb, err := alpmHandle.LocalDb()
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
			fmt.Printf("%s: \x1B[0;33m%s\x1B[0m\n", pkg.Name(), human(pkg.ISize()))

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

	localDb, err := alpmHandle.LocalDb()
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
	localDb, err := alpmHandle.LocalDb()
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
				fmt.Printf("%s: \x1B[0;33m%s\x1B[0m\n", pkg.Name(), human(pkg.ISize()))
			}
		}
	}
	return
}

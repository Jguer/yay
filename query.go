package main

import (
	"fmt"
	"sort"
	"strings"
	"sync"

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
func syncInfo(pkgS []string) (err error) {
	var info []rpc.Pkg
	aurS, repoS, err := packageSlices(pkgS)
	if err != nil {
		return
	}

	if len(aurS) != 0 {
		noDb := make([]string, 0, len(aurS))

		for _, pkg := range aurS {
			_, name := splitDbFromName(pkg)
			noDb = append(noDb, name)
		}

		info, err = aurInfo(noDb)
		if err != nil {
			fmt.Println(err)
		}
	}

	// Repo always goes first
	if len(repoS) != 0 {
		arguments := cmdArgs.copy()
		arguments.delTarget(aurS...)
		err = passToPacman(arguments)

		if err != nil {
			return
		}
	}

	if len(aurS) != 0 {
		for _, pkg := range info {
			PrintInfo(&pkg)
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

	for _, _pkg := range toCheck {
		db, name := splitDbFromName(_pkg)
		found := false

		if db == "aur" {
			aur = append(aur, _pkg)
			continue
		} else if db != "" {
			repo = append(repo, _pkg)
			continue
		}

		_ = dbList.ForEach(func(db alpm.Db) error {
			_, err := db.PkgByName(name)

			if err == nil {
				found = true
				return fmt.Errorf("")

			}
			return nil
		})

		if !found {
			_, errdb := dbList.PkgCachebyGroup(name)
			found = errdb == nil
		}

		if found {
			repo = append(repo, _pkg)
		} else {
			aur = append(aur, _pkg)
		}
	}

	return
}

// HangingPackages returns a list of packages installed as deps
// and unneeded by the system
// removeOptional decides whether optional dependencies are counted or not
func hangingPackages(removeOptional bool) (hanging []string, err error) {
	localDb, err := alpmHandle.LocalDb()
	if err != nil {
		return
	}

	// safePackages represents every package in the system in one of 3 states
	// State = 0 - Remove package from the system
	// State = 1 - Keep package in the system; need to iterate over dependencies
	// State = 2 - Keep package and have iterated over dependencies
	safePackages := make(map[string]uint8)
	// provides stores a mapping from the provides name back to the original package name
	// Assumption - multiple installed packages don't provide the same dependency
	provides := make(map[string]string)
	packages := localDb.PkgCache()

	// Mark explicit dependencies and enumerate the provides list
	setupResources := func(pkg alpm.Package) error {
		if pkg.Reason() == alpm.PkgReasonExplicit {
			safePackages[pkg.Name()] = 1
		} else {
			safePackages[pkg.Name()] = 0
		}

		pkg.Provides().ForEach(func(dep alpm.Depend) error {
			provides[dep.Name] = pkg.Name()
			return nil
		})
		return nil
	}
	packages.ForEach(setupResources)

	iterateAgain := true
	processDependencies := func(pkg alpm.Package) error {
		if state, _ := safePackages[pkg.Name()]; state == 0 || state == 2 {
			return nil
		}

		safePackages[pkg.Name()] = 2

		// Update state for dependencies
		markDependencies := func(dep alpm.Depend) error {
			// Don't assume a dependency is installed
			state, ok := safePackages[dep.Name]
			if !ok {
				// Check if dep is a provides rather than actual package name
				if p, ok2 := provides[dep.Name]; ok2 {
					iterateAgain = true
					safePackages[p] = 1
				}

				return nil
			}

			if state == 0 {
				iterateAgain = true
				safePackages[dep.Name] = 1
			}
			return nil
		}

		pkg.Depends().ForEach(markDependencies)
		if !removeOptional {
			pkg.OptionalDepends().ForEach(markDependencies)
		}
		return nil
	}

	for iterateAgain {
		iterateAgain = false
		packages.ForEach(processDependencies)
	}

	// Build list of packages to be removed
	packages.ForEach(func(pkg alpm.Package) error {
		if safePackages[pkg.Name()] == 0 {
			hanging = append(hanging, pkg.Name())
		}
		return nil
	})

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

// Queries the aur for information about specified packages.
// All packages should be queried in a single rpc request except when the number
// of packages exceeds the number set in config.RequestSplitN.
// If the number does exceed config.RequestSplitN multiple rpc requests will be
// performed concurrently.
func aurInfo(names []string) ([]rpc.Pkg, error) {
	info := make([]rpc.Pkg, 0, len(names))
	seen := make(map[string]int)
	var mux sync.Mutex
	var wg sync.WaitGroup
	var err error

	missing := make([]string, 0, len(names))
	orphans := make([]string, 0, len(names))
	outOfDate := make([]string, 0, len(names))

	makeRequest := func(n, max int) {
		defer wg.Done()
		tempInfo, requestErr := rpc.Info(names[n:max])
		if err != nil {
			return
		}
		if requestErr != nil {
			err = requestErr
			return
		}
		mux.Lock()
		info = append(info, tempInfo...)
		mux.Unlock()
	}

	for n := 0; n < len(names); n += config.RequestSplitN {
		max := min(len(names), n+config.RequestSplitN)
		wg.Add(1)
		go makeRequest(n, max)
	}

	wg.Wait()

	if err != nil {
		return info, err
	}

	for k, pkg := range info {
		seen[pkg.Name] = k
	}

	for _, name := range names {
		i, ok := seen[name]
		if !ok {
			missing = append(missing, name)
			continue
		}

		pkg := info[i]

		if pkg.Maintainer == "" {
			orphans = append(orphans, name)
		}
		if pkg.OutOfDate != 0 {
			outOfDate = append(outOfDate, name)
		}
	}

	if len(missing) > 0 {
		fmt.Print(bold(red(arrow + " Missing AUR Packages:")))
		for _, name := range missing {
			fmt.Print(" " + bold(magenta(name)))
		}
		fmt.Println()
	}

	if len(orphans) > 0 {
		fmt.Print(bold(red(arrow + " Orphaned AUR Packages:")))
		for _, name := range orphans {
			fmt.Print(" " + bold(magenta(name)))
		}
		fmt.Println()
	}

	if len(outOfDate) > 0 {
		fmt.Print(bold(red(arrow + " Out Of Date AUR Packages:")))
		for _, name := range outOfDate {
			fmt.Print(" " + bold(magenta(name)))
		}
		fmt.Println()
	}

	return info, nil
}

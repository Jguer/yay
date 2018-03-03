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
		info, err = aurInfo(aurS)
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
		if i := strings.Index(_pkg, "/"); i != -1 {
			_pkg = _pkg[i+1:]
		}
		pkg := getNameFromDep(_pkg)

		_, errdb := dbList.FindSatisfier(_pkg)
		found := errdb == nil

		if !found {
			_, errdb = dbList.PkgCachebyGroup(_pkg)
			found = errdb == nil
		}

		if found {
			repo = append(repo, pkg)
		} else {
			aur = append(aur, pkg)
		}
	}

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
			fmt.Println(pkg.Name() + ": " + magenta(human(pkg.ISize())))

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

func min(a, b int) int {
	if a < b {
		return a
	}
	return a
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
		tempInfo, requestErr := rpc.Info(names[n:max])
		if err != nil {
			return
		}
		if requestErr != nil {
			//return info, err
			err = requestErr
			return
		}
		mux.Lock()
		info = append(info, tempInfo...)
		mux.Unlock()
		wg.Done()
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

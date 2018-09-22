package main

import (
	"sort"
	"strings"
	"sync"

	alpm "github.com/jguer/go-alpm"
	rpc "github.com/mikkeloscar/aur"
)

type depSolver struct {
	Aur      []Base
	Repo     []*alpm.Package
	Runtime  stringSet
	Targets  []target
	Explicit stringSet
	AurCache map[string]*rpc.Pkg
	Groups   []string
	LocalDb  *alpm.Db
	SyncDb   alpm.DbList
	Seen     stringSet
	Warnings *aurWarnings
}

func makeDepSolver() (*depSolver, error) {
	localDb, err := alpmHandle.LocalDb()
	if err != nil {
		return nil, err
	}
	syncDb, err := alpmHandle.SyncDbs()
	if err != nil {
		return nil, err
	}

	return &depSolver{
		make([]Base, 0),
		make([]*alpm.Package, 0),
		make(stringSet),
		make([]target, 0),
		make(stringSet),
		make(map[string]*rpc.Pkg),
		make([]string, 0),
		localDb,
		syncDb,
		make(stringSet),
		nil,
	}, nil
}

func getDepSolver(pkgs []string, warnings *aurWarnings) (*depSolver, error) {
	ds, err := makeDepSolver()
	if err != nil {
		return nil, err
	}

	ds.Warnings = warnings
	err = ds.resolveTargets(pkgs)
	if err != nil {
		return nil, err
	}

	ds.resolveRuntime()
	return ds, err
}

// Includes db/ prefixes and group installs
func (ds *depSolver) resolveTargets(pkgs []string) error {
	// RPC requests are slow
	// Combine as many AUR package requests as possible into a single RPC
	// call
	aurTargets := make([]string, 0)
	pkgs = removeInvalidTargets(pkgs)

	for _, pkg := range pkgs {
		var err error
		target := toTarget(pkg)

		// skip targets already satisfied
		// even if the user enters db/pkg and aur/pkg the latter will
		// still get skipped even if it's from a different database to
		// the one specified
		// this is how pacman behaves
		if ds.hasPackage(target.DepString()) {
			continue
		}

		var foundPkg *alpm.Package
		var singleDb *alpm.Db

		// aur/ prefix means we only check the aur
		if target.Db == "aur" || config.mode == modeAUR {
			ds.Targets = append(ds.Targets, target)
			aurTargets = append(aurTargets, target.DepString())
			continue
		}

		// If there'ss a different priefix only look in that repo
		if target.Db != "" {
			singleDb, err = alpmHandle.SyncDbByName(target.Db)
			if err != nil {
				return err
			}
			foundPkg, err = singleDb.PkgCache().FindSatisfier(target.DepString())
			//otherwise find it in any repo
		} else {
			foundPkg, err = ds.SyncDb.FindSatisfier(target.DepString())
		}

		if err == nil {
			ds.Targets = append(ds.Targets, target)
			ds.Explicit.set(foundPkg.Name())
			ds.ResolveRepoDependency(foundPkg)
			continue
		} else {
			//check for groups
			//currently we don't resolve the packages in a group
			//only check if the group exists
			//would be better to check the groups from singleDb if
			//the user specified a db but there's no easy way to do
			//it without making alpm_lists so don't bother for now
			//db/group is probably a rare use case
			group, err := ds.SyncDb.PkgCachebyGroup(target.Name)
			if err == nil {
				ds.Groups = append(ds.Groups, target.String())
				group.ForEach(func(pkg alpm.Package) error {
					ds.Explicit.set(pkg.Name())
					return nil
				})
				continue
			}
		}

		//if there was no db prefix check the aur
		if target.Db == "" {
			aurTargets = append(aurTargets, target.DepString())
		}

		ds.Targets = append(ds.Targets, target)
	}

	if len(aurTargets) > 0 && (config.mode == modeAny || config.mode == modeAUR) {
		return ds.resolveAURPackages(aurTargets, true)
	}

	return nil
}

func (ds *depSolver) hasPackage(name string) bool {
	for _, pkg := range ds.Repo {
		if pkg.Name() == name {
			return true
		}
	}

	for _, base := range ds.Aur {
		for _, pkg := range base {
			if pkg.Name == name {
				return true
			}
		}
	}

	for _, pkg := range ds.Groups {
		if pkg == name {
			return true
		}
	}

	return false
}

func (ds *depSolver) findSatisfierAur(dep string) *rpc.Pkg {
	for _, base := range ds.Aur {
		for _, pkg := range base {
			if satisfiesAur(dep, pkg) {
				return pkg
			}
		}
	}

	return nil
}

func (ds *depSolver) findSatisfierRepo(dep string) *alpm.Package {
	for _, pkg := range ds.Repo {
		if satisfiesRepo(dep, pkg) {
			return pkg
		}
	}

	return nil
}

func (ds *depSolver) hasSatisfier(dep string) bool {
	return ds.findSatisfierRepo(dep) != nil || ds.findSatisfierAur(dep) != nil
}

func (ds *depSolver) ResolveRepoDependency(pkg *alpm.Package) {
	if ds.Seen.get(pkg.Name()) {
		return
	}
	ds.Repo = append(ds.Repo, pkg)
	ds.Seen.set(pkg.Name())

	pkg.Depends().ForEach(func(dep alpm.Depend) (err error) {
		//have satisfier in dep tree: skip
		if ds.hasSatisfier(dep.String()) {
			return
		}

		//has satisfier installed: skip
		_, isInstalled := ds.LocalDb.PkgCache().FindSatisfier(dep.String())
		if isInstalled == nil {
			return
		}

		//has satisfier in repo: fetch it
		repoPkg, inRepos := ds.SyncDb.FindSatisfier(dep.String())
		if inRepos != nil {
			return
		}

		ds.ResolveRepoDependency(repoPkg)
		return nil
	})
}

// This is mostly used to promote packages from the cache
// to the Install list
// Provide a pacman style provider menu if there's more than one candidate
// This acts slightly differently from Pacman, It will give
// a menu even if a package with a matching name exists. I believe this
// method is better because most of the time you are choosing between
// foo and foo-git.
// Using Pacman's ways trying to install foo would never give you
// a menu.
// TODO: maybe intermix repo providers in the menu
func (ds *depSolver) findSatisfierAurCache(dep string) *rpc.Pkg {
	depName, _, _ := splitDep(dep)
	seen := make(stringSet)
	providers := makeProviders(depName)

	if _, err := ds.LocalDb.PkgByName(depName); err == nil {
		if pkg, ok := ds.AurCache[dep]; ok && pkgSatisfies(pkg.Name, pkg.Version, dep) {
			return pkg
		}

	}

	if cmdArgs.op == "Y" || cmdArgs.op == "yay" {
		for _, pkg := range ds.AurCache {
			if pkgSatisfies(pkg.Name, pkg.Version, dep) {
				for _, target := range ds.Targets {
					if target.Name == pkg.Name {
						return pkg
					}
				}
			}
		}
	}

	for _, pkg := range ds.AurCache {
		if seen.get(pkg.Name) {
			continue
		}

		if pkgSatisfies(pkg.Name, pkg.Version, dep) {
			providers.Pkgs = append(providers.Pkgs, pkg)
			seen.set(pkg.Name)
			continue
		}

		for _, provide := range pkg.Provides {
			if provideSatisfies(provide, dep) {
				providers.Pkgs = append(providers.Pkgs, pkg)
				seen.set(pkg.Name)
				continue
			}
		}
	}

	if !config.boolean["provides"] && providers.Len() >= 1 {
		return providers.Pkgs[0]
	}

	if providers.Len() == 1 {
		return providers.Pkgs[0]
	}

	if providers.Len() > 1 {
		sort.Sort(providers)
		return providerMenu(dep, providers)
	}

	return nil
}

func (ds *depSolver) cacheAURPackages(_pkgs []string) error {
	pkgs := sliceToStringSet(_pkgs)
	query := make([]string, 0)

	for pkg := range pkgs {
		if _, ok := ds.AurCache[pkg]; ok {
			pkgs.remove(pkg)
		}
	}

	if len(pkgs) == 0 {
		return nil
	}

	if config.boolean["provides"] {
		err := ds.findProvides(pkgs)
		if err != nil {
			return err
		}
	}

	for pkg := range pkgs {
		if _, ok := ds.AurCache[pkg]; !ok {
			name, _, _ := splitDep(pkg)
			query = append(query, name)
		}
	}

	info, err := aurInfo(query, ds.Warnings)
	if err != nil {
		return err
	}

	for _, pkg := range info {
		// Dump everything in cache just in case we need it later
		ds.AurCache[pkg.Name] = pkg
	}

	return nil
}

// Pseudo provides finder.
// Try to find provides by performing a search of the package name
// This effectively performs -Ss on each package
// then runs -Si on each result to cache the information.
//
// For example if you were to -S yay then yay -Ss would give:
// yay-git yay-bin yay realyog pacui pacui-git ruby-yard
// These packages will all be added to the cache in case they are needed later
// Ofcouse only the first three packages provide yay, the rest are just false
// positives.
//
// This method increases dependency resolve time
func (ds *depSolver) findProvides(pkgs stringSet) error {
	var mux sync.Mutex
	var wg sync.WaitGroup

	doSearch := func(pkg string) {
		defer wg.Done()
		var err error
		var results []rpc.Pkg

		// Hack for a bigger search result, if the user wants
		// java-envronment we can search for just java instead and get
		// more hits.
		words := strings.Split(pkg, "-")

		for i := range words {
			results, err = rpc.SearchByNameDesc(strings.Join(words[:i+1], "-"))
			if err == nil {
				break
			}
		}

		if err != nil {
			return
		}

		for _, result := range results {
			mux.Lock()
			if _, ok := ds.AurCache[result.Name]; !ok {
				pkgs.set(result.Name)
			}
			mux.Unlock()
		}
	}

	for pkg := range pkgs {
		if _, err := ds.LocalDb.PkgByName(pkg); err == nil {
			continue
		}
		wg.Add(1)
		go doSearch(pkg)
	}

	wg.Wait()

	return nil
}

func (ds *depSolver) resolveAURPackages(pkgs []string, explicit bool) error {
	newPackages := make(stringSet)
	newAURPackages := make([]string, 0)
	toAdd := make([]*rpc.Pkg, 0)

	if len(pkgs) == 0 {
		return nil
	}

	err := ds.cacheAURPackages(pkgs)
	if err != nil {
		return err
	}

	for _, name := range pkgs {
		if ds.Seen.get(name) {
			continue
		}

		pkg := ds.findSatisfierAurCache(name)
		if pkg == nil {
			continue
		}

		if explicit {
			ds.Explicit.set(pkg.Name)
		}

		ds.Seen.set(pkg.Name)
		toAdd = append(toAdd, pkg)

		for _, deps := range [3][]string{pkg.Depends, pkg.MakeDepends, pkg.CheckDepends} {
			for _, dep := range deps {
				newPackages.set(dep)
			}
		}
	}

	for dep := range newPackages {
		if ds.hasSatisfier(dep) {
			continue
		}

		_, isInstalled := ds.LocalDb.PkgCache().FindSatisfier(dep) //has satisfier installed: skip
		hm := config.hideMenus
		config.hideMenus = isInstalled == nil
		repoPkg, inRepos := ds.SyncDb.FindSatisfier(dep) //has satisfier in repo: fetch it
		config.hideMenus = hm
		if isInstalled == nil && (config.value["rebuild"] != "tree" || inRepos == nil) {
			continue
		}

		if inRepos == nil {
			ds.ResolveRepoDependency(repoPkg)
			continue
		}

		//assume it's in the aur
		//ditch the versioning because the RPC can't handle it
		newAURPackages = append(newAURPackages, dep)

	}

	err = ds.resolveAURPackages(newAURPackages, false)

	for _, pkg := range toAdd {
		if !ds.hasPackage(pkg.Name) {
			ds.Aur = baseAppend(ds.Aur, pkg)
		}
	}

	return err
}

func (ds *depSolver) Print() {
	repo := ""
	repoMake := ""
	aur := ""
	aurMake := ""

	repoLen := 0
	repoMakeLen := 0
	aurLen := 0
	aurMakeLen := 0

	for _, pkg := range ds.Repo {
		if ds.Runtime.get(pkg.Name()) {
			repo += "  " + pkg.Name() + "-" + pkg.Version()
			repoLen++
		} else {
			repoMake += "  " + pkg.Name() + "-" + pkg.Version()
			repoMakeLen++
		}
	}

	for _, base := range ds.Aur {
		pkg := base.Pkgbase()
		pkgStr := "  " + pkg + "-" + base[0].Version
		pkgStrMake := pkgStr

		push := false
		pushMake := false

		if len(base) > 1 || pkg != base[0].Name {
			pkgStr += " ("
			pkgStrMake += " ("

			for _, split := range base {
				if ds.Runtime.get(split.Name) {
					pkgStr += split.Name + " "
					aurLen++
					push = true
				} else {
					pkgStrMake += split.Name + " "
					aurMakeLen++
					pushMake = true
				}
			}

			pkgStr = pkgStr[:len(pkgStr)-1] + ")"
			pkgStrMake = pkgStrMake[:len(pkgStrMake)-1] + ")"
		} else if ds.Runtime.get(base[0].Name) {
			aurLen++
			push = true
		} else {
			aurMakeLen++
			pushMake = true
		}

		if push {
			aur += pkgStr
		}
		if pushMake {
			aurMake += pkgStrMake
		}
	}

	printDownloads("Repo", repoLen, repo)
	printDownloads("Repo Make", repoMakeLen, repoMake)
	printDownloads("Aur", aurLen, aur)
	printDownloads("Aur Make", aurMakeLen, aurMake)
}

func (ds *depSolver) resolveRuntime() {
	for _, pkg := range ds.Repo {
		if ds.Explicit.get(pkg.Name()) {
			ds.Runtime.set(pkg.Name())
			ds.resolveRuntimeRepo(pkg)
		}
	}

	for _, base := range ds.Aur {
		for _, pkg := range base {
			if ds.Explicit.get(pkg.Name) {
				ds.Runtime.set(pkg.Name)
				ds.resolveRuntimeAur(pkg)
			}
		}
	}
}

func (ds *depSolver) resolveRuntimeRepo(pkg *alpm.Package) {
	pkg.Depends().ForEach(func(dep alpm.Depend) (err error) {
		for _, pkg := range ds.Repo {
			if ds.Runtime.get(pkg.Name()) {
				continue
			}

			if satisfiesRepo(dep.String(), pkg) {
				ds.Runtime.set(pkg.Name())
				ds.resolveRuntimeRepo(pkg)
			}
		}
		return nil
	})
}

func (ds *depSolver) resolveRuntimeAur(pkg *rpc.Pkg) {
	for _, dep := range pkg.Depends {
		for _, pkg := range ds.Repo {
			if ds.Runtime.get(pkg.Name()) {
				continue
			}

			if satisfiesRepo(dep, pkg) {
				ds.Runtime.set(pkg.Name())
				ds.resolveRuntimeRepo(pkg)
			}
		}

		for _, base := range ds.Aur {
			for _, pkg := range base {
				if ds.Runtime.get(pkg.Name) {
					continue
				}

				if satisfiesAur(dep, pkg) {
					ds.Runtime.set(pkg.Name)
					ds.resolveRuntimeAur(pkg)
				}
			}
		}
	}
}

func (ds *depSolver) HasMake() bool {
	lenAur := 0
	for _, base := range ds.Aur {
		lenAur += len(base)
	}

	return len(ds.Runtime) != lenAur+len(ds.Repo)
}

func (ds *depSolver) getMake() []string {
	makeOnly := make([]string, 0, len(ds.Aur)+len(ds.Repo)-len(ds.Runtime))

	for _, base := range ds.Aur {
		for _, pkg := range base {
			if !ds.Runtime.get(pkg.Name) {
				makeOnly = append(makeOnly, pkg.Name)
			}
		}
	}

	for _, pkg := range ds.Repo {
		if !ds.Runtime.get(pkg.Name()) {
			makeOnly = append(makeOnly, pkg.Name())
		}
	}

	return makeOnly
}

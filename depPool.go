package main

import (
	"sort"
	"strings"
	"sync"

	alpm "github.com/jguer/go-alpm"
	rpc "github.com/mikkeloscar/aur"
)

type target struct {
	Db      string
	Name    string
	Mod     string
	Version string
}

func toTarget(pkg string) target {
	db, dep := splitDbFromName(pkg)
	name, mod, version := splitDep(dep)

	return target{
		db,
		name,
		mod,
		version,
	}
}

func (t target) DepString() string {
	return t.Name + t.Mod + t.Version
}

func (t target) String() string {
	if t.Db != "" {
		return t.Db + "/" + t.DepString()
	}

	return t.DepString()
}

type depPool struct {
	Targets  []target
	Explicit stringSet
	Repo     map[string]*alpm.Package
	Aur      map[string]*rpc.Pkg
	AurCache map[string]*rpc.Pkg
	Groups   []string
	LocalDb  *alpm.Db
	SyncDb   alpm.DbList
	Warnings *aurWarnings
}

func makeDepPool() (*depPool, error) {
	localDb, err := alpmHandle.LocalDb()
	if err != nil {
		return nil, err
	}
	syncDb, err := alpmHandle.SyncDbs()
	if err != nil {
		return nil, err
	}

	dp := &depPool{
		make([]target, 0),
		make(stringSet),
		make(map[string]*alpm.Package),
		make(map[string]*rpc.Pkg),
		make(map[string]*rpc.Pkg),
		make([]string, 0),
		localDb,
		syncDb,
		nil,
	}

	return dp, nil
}

// Includes db/ prefixes and group installs
func (dp *depPool) ResolveTargets(pkgs []string) error {
	// RPC requests are slow
	// Combine as many AUR package requests as possible into a single RPC
	// call
	aurTargets := make(stringSet)

	pkgs = removeInvalidTargets(pkgs)

	for _, pkg := range pkgs {
		var err error
		target := toTarget(pkg)

		// skip targets already satisfied
		// even if the user enters db/pkg and aur/pkg the latter will
		// still get skipped even if it's from a different database to
		// the one specified
		// this is how pacman behaves
		if dp.hasPackage(target.DepString()) {
			continue
		}

		var foundPkg *alpm.Package
		var singleDb *alpm.Db

		// aur/ prefix means we only check the aur
		if target.Db == "aur" || mode == ModeAUR {
			dp.Targets = append(dp.Targets, target)
			aurTargets.set(target.DepString())
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
			foundPkg, err = dp.SyncDb.FindSatisfier(target.DepString())
		}

		if err == nil {
			dp.Targets = append(dp.Targets, target)
			dp.Explicit.set(foundPkg.Name())
			dp.ResolveRepoDependency(foundPkg)
			continue
		} else {
			//check for groups
			//currently we don't resolve the packages in a group
			//only check if the group exists
			//would be better to check the groups from singleDb if
			//the user specified a db but there's no easy way to do
			//it without making alpm_lists so don't bother for now
			//db/group is probably a rare use case
			group, err := dp.SyncDb.PkgCachebyGroup(target.Name)
			if err == nil {
				dp.Groups = append(dp.Groups, target.String())
				group.ForEach(func(pkg alpm.Package) error {
					dp.Explicit.set(pkg.Name())
					return nil
				})
				continue
			}
		}

		//if there was no db prefix check the aur
		if target.Db == "" {
			aurTargets.set(target.DepString())
		}

		dp.Targets = append(dp.Targets, target)
	}

	if len(aurTargets) > 0 && (mode == ModeAny || mode == ModeAUR) {
		return dp.resolveAURPackages(aurTargets, true)
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
func (dp *depPool) findProvides(pkgs stringSet) error {
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
			if _, ok := dp.AurCache[result.Name]; !ok {
				pkgs.set(result.Name)
			}
			mux.Unlock()
		}
	}

	for pkg := range pkgs {
		if _, err := dp.LocalDb.PkgByName(pkg); err == nil {
			continue
		}
		wg.Add(1)
		go doSearch(pkg)
	}

	wg.Wait()

	return nil
}

func (dp *depPool) cacheAURPackages(_pkgs stringSet) error {
	pkgs := _pkgs.copy()
	query := make([]string, 0)

	for pkg := range pkgs {
		if _, ok := dp.AurCache[pkg]; ok {
			pkgs.remove(pkg)
		}
	}

	if len(pkgs) == 0 {
		return nil
	}

	if config.Provides {
		err := dp.findProvides(pkgs)
		if err != nil {
			return err
		}
	}

	for pkg := range pkgs {
		if _, ok := dp.AurCache[pkg]; !ok {
			name, _, _ := splitDep(pkg)
			query = append(query, name)
		}
	}

	info, err := aurInfo(query, dp.Warnings)
	if err != nil {
		return err
	}

	for _, pkg := range info {
		// Dump everything in cache just in case we need it later
		dp.AurCache[pkg.Name] = pkg
	}

	return nil
}

func (dp *depPool) resolveAURPackages(pkgs stringSet, explicit bool) error {
	newPackages := make(stringSet)
	newAURPackages := make(stringSet)

	err := dp.cacheAURPackages(pkgs)
	if err != nil {
		return err
	}

	if len(pkgs) == 0 {
		return nil
	}

	for name := range pkgs {
		_, ok := dp.Aur[name]
		if ok {
			continue
		}

		pkg := dp.findSatisfierAurCache(name)
		if pkg == nil {
			continue
		}

		if explicit {
			dp.Explicit.set(pkg.Name)
		}
		dp.Aur[pkg.Name] = pkg

		for _, deps := range [3][]string{pkg.Depends, pkg.MakeDepends, pkg.CheckDepends} {
			for _, dep := range deps {
				newPackages.set(dep)
			}
		}
	}

	for dep := range newPackages {
		if dp.hasSatisfier(dep) {
			continue
		}

		//has satisfier installed: skip
		_, isInstalled := dp.LocalDb.PkgCache().FindSatisfier(dep)
		if isInstalled == nil {
			continue
		}

		//has satisfier in repo: fetch it
		repoPkg, inRepos := dp.SyncDb.FindSatisfier(dep)
		if inRepos == nil {
			dp.ResolveRepoDependency(repoPkg)
			continue
		}

		//assume it's in the aur
		//ditch the versioning because the RPC can't handle it
		newAURPackages.set(dep)

	}

	err = dp.resolveAURPackages(newAURPackages, false)

	return err
}

func (dp *depPool) ResolveRepoDependency(pkg *alpm.Package) {
	dp.Repo[pkg.Name()] = pkg

	pkg.Depends().ForEach(func(dep alpm.Depend) (err error) {
		//have satisfier in dep tree: skip
		if dp.hasSatisfier(dep.String()) {
			return
		}

		//has satisfier installed: skip
		_, isInstalled := dp.LocalDb.PkgCache().FindSatisfier(dep.String())
		if isInstalled == nil {
			return
		}

		//has satisfier in repo: fetch it
		repoPkg, inRepos := dp.SyncDb.FindSatisfier(dep.String())
		if inRepos != nil {
			return
		}

		dp.ResolveRepoDependency(repoPkg)

		return nil
	})
}

func getDepPool(pkgs []string, warnings *aurWarnings) (*depPool, error) {
	dp, err := makeDepPool()
	if err != nil {
		return nil, err
	}

	dp.Warnings = warnings
	err = dp.ResolveTargets(pkgs)

	return dp, err
}

func (dp *depPool) findSatisfierAur(dep string) *rpc.Pkg {
	for _, pkg := range dp.Aur {
		if satisfiesAur(dep, pkg) {
			return pkg
		}
	}

	return nil
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
func (dp *depPool) findSatisfierAurCache(dep string) *rpc.Pkg {
	depName, _, _ := splitDep(dep)
	seen := make(stringSet)
	providers := makeProviders(depName)

	if _, err := dp.LocalDb.PkgByName(depName); err == nil {
		if pkg, ok := dp.AurCache[dep]; ok && pkgSatisfies(pkg.Name, pkg.Version, dep) {
			return pkg
		}

	}

	if cmdArgs.op == "Y" || cmdArgs.op == "yay" {
		for _, pkg := range dp.AurCache {
			if pkgSatisfies(pkg.Name, pkg.Version, dep) {
				for _, target := range dp.Targets {
					if target.Name == pkg.Name {
						return pkg
					}
				}
			}
		}
	}

	for _, pkg := range dp.AurCache {
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

	if providers.Len() == 1 {
		return providers.Pkgs[0]
	}

	if providers.Len() > 1 {
		sort.Sort(providers)
		return providerMenu(dep, providers)
	}

	return nil
}

func (dp *depPool) findSatisfierRepo(dep string) *alpm.Package {
	for _, pkg := range dp.Repo {
		if satisfiesRepo(dep, pkg) {
			return pkg
		}
	}

	return nil
}

func (dp *depPool) hasSatisfier(dep string) bool {
	return dp.findSatisfierRepo(dep) != nil || dp.findSatisfierAur(dep) != nil
}

func (dp *depPool) hasPackage(name string) bool {
	for _, pkg := range dp.Repo {
		if pkg.Name() == name {
			return true
		}
	}

	for _, pkg := range dp.Aur {
		if pkg.Name == name {
			return true
		}
	}

	for _, pkg := range dp.Groups {
		if pkg == name {
			return true
		}
	}

	return false
}

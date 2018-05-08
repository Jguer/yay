package main

import (
	"fmt"
	"strings"
	"sync"

	alpm "github.com/jguer/go-alpm"
	rpc "github.com/mikkeloscar/aur"
)

const PROVIDES = false

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
		make(map[string]*alpm.Package),
		make(map[string]*rpc.Pkg),
		make(map[string]*rpc.Pkg),
		make([]string, 0),
		localDb,
		syncDb,
		&aurWarnings{},
	}

	return dp, nil
}

// Includes db/ prefixes and group installs
func (dp *depPool) ResolveTargets(pkgs []string) error {
	for _, pkg := range pkgs {
		target := toTarget(pkg)
		dp.Targets = append(dp.Targets, target)
	}

	// RPC requests are slow
	// Combine as many AUR package requests as possible into a single RPC
	// call
	aurTargets := make(stringSet)
	var err error
	//repo := make([]*alpm.Package, 0)

	for _, target := range dp.Targets {

		// skip targets already satisfied
		// even if the user enters db/pkg and aur/pkg the latter will
		// still get skiped even if it's from a different database to
		// the one specified
		// this is how pacman behaves
		if dp.hasSatisfier(target.DepString()) {
			fmt.Println("Skipping target", target)
			continue
		}

		var foundPkg *alpm.Package
		var singleDb *alpm.Db

		// aur/ prefix means we only check the aur
		if target.Db == "aur" {
			aurTargets.set(target.DepString())
			continue
		}

		// if theres a different priefix only look in that repo
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
			dp.ResolveRepoDependency(foundPkg)
			continue
		} else {
			//check for groups
			//currently we dont resolve the packages in a group
			//only check if the group exists
			//would be better to check the groups from singleDb if
			//the user specified a db but theres no easy way to do
			//it without making alpm_lists so dont bother for now
			//db/group is probably a rare use case
			_, err := dp.SyncDb.PkgCachebyGroup(target.Name)

			if err == nil {
				dp.Groups = append(dp.Groups, target.String())
				continue
			}
		}

		//if there was no db prefix check the aur
		if target.Db == "" {
			aurTargets.set(target.DepString())
		}
	}

	if len(aurTargets) > 0 {
		err = dp.resolveAURPackages(aurTargets)
	}

	return err
}

// Pseudo provides finder.
// Try to find provides by performing a search of the package name
// This effectively performs -Ss on each package
// then runs -Si on each result to cache the information.
//
// For example if you were to -S yay then yay -Ss would give:
// yay-git yay-bin yay realyog pacui pacui-git ruby-yard
// These packages will all be added to the cache incase they are needed later
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

	//TODO: config option, maybe --deepsearh but aurman uses that flag for
	//something else already which might be confusing
	//maybe --provides
	if PROVIDES {
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

func (dp *depPool) resolveAURPackages(pkgs stringSet) error {
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
		//ditch the versioning because the RPC cant handle it
		newAURPackages.set(dep)

	}

	err = dp.resolveAURPackages(newAURPackages)

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

func getDepPool(pkgs []string) (*depPool, error) {
	dp, err := makeDepPool()
	if err != nil {
		return nil, err
	}

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
// Provide a pacman style provider menu if theres more than one candidate
// TODO: maybe intermix repo providers in the menu
func (dp *depPool) findSatisfierAurCache(dep string) *rpc.Pkg {
	//try to match providers
	providers := make([]*rpc.Pkg, 0)
	for _, pkg := range dp.AurCache {
		if pkgSatisfies(pkg.Name, pkg.Version, dep) {
			return pkg
		}
	}

	for _, pkg := range dp.AurCache {
		for _, provide := range pkg.Provides {
			if provideSatisfies(provide, dep) {
				providers = append(providers, pkg)
			}
		}
	}

	if len(providers) == 1 {
		return providers[0]
	}

	if len(providers) > 1 {
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


package main

import (
	alpm "github.com/jguer/go-alpm"
	rpc "github.com/mikkeloscar/aur"
	"strings"
)

type depTree struct {
	ToProcess stringSet
	Repo      map[string]*alpm.Package
	Aur       map[string]*rpc.Pkg
	Missing   stringSet
}

type depCatagories struct {
	Repo     []*alpm.Package
	Aur      []*rpc.Pkg
	MakeOnly stringSet
	Bases    map[string][]*rpc.Pkg
}

func makeDepTree() *depTree {
	dt := depTree{
		make(stringSet),
		make(map[string]*alpm.Package),
		make(map[string]*rpc.Pkg),
		make(stringSet),
	}

	return &dt
}

func makeDependCatagories() *depCatagories {
	dc := depCatagories{
		make([]*alpm.Package, 0),
		make([]*rpc.Pkg, 0),
		make(stringSet),
		make(map[string][]*rpc.Pkg),
	}

	return &dc
}

func getNameFromDep(dep string) string {
	return strings.FieldsFunc(dep, func(c rune) bool {
		return c == '>' || c == '<' || c == '='
	})[0]
}

func getDepCatagories(pkgs []string, dt *depTree) (*depCatagories, error) {
	dc := makeDependCatagories()
	seen := make(stringSet)

	for _, pkg := range pkgs {
		dep := getNameFromDep(pkg)
		alpmpkg, exists := dt.Repo[dep]
		if exists {
			repoDepCatagoriesRecursive(alpmpkg, dc, dt, false)
			dc.Repo = append(dc.Repo, alpmpkg)
			delete(dt.Repo, dep)
		}

		aurpkg, exists := dt.Aur[dep]
		if exists {
			depCatagoriesRecursive(aurpkg, dc, dt, false, seen)
			if !seen.get(aurpkg.PackageBase) {
				dc.Aur = append(dc.Aur, aurpkg)
				seen.set(aurpkg.PackageBase)
			}

			_, ok := dc.Bases[aurpkg.PackageBase]
			if !ok {
				dc.Bases[aurpkg.PackageBase] = make([]*rpc.Pkg, 0)
			}
			dc.Bases[aurpkg.PackageBase] = append(dc.Bases[aurpkg.PackageBase], aurpkg)
			delete(dt.Aur, dep)
		}
	}

	for _, base := range dc.Bases {
		for _, pkg := range base {
			for _, dep := range pkg.Depends {
				dc.MakeOnly.remove(dep)
			}
		}
	}

	for _, pkg := range dc.Repo {
		pkg.Depends().ForEach(func(_dep alpm.Depend) error {
			dep := _dep.Name
			dc.MakeOnly.remove(dep)

			return nil
		})
	}

	return dc, nil
}

func repoDepCatagoriesRecursive(pkg *alpm.Package, dc *depCatagories, dt *depTree, isMake bool) {
	pkg.Depends().ForEach(func(_dep alpm.Depend) error {
		dep := _dep.Name
		alpmpkg, exists := dt.Repo[dep]
		if exists {
			delete(dt.Repo, dep)
			repoDepCatagoriesRecursive(alpmpkg, dc, dt, isMake)

			if isMake {
				dc.MakeOnly.set(alpmpkg.Name())
			}

			dc.Repo = append(dc.Repo, alpmpkg)
		}

		return nil
	})
}

func depCatagoriesRecursive(pkg *rpc.Pkg, dc *depCatagories, dt *depTree, isMake bool, seen stringSet) {
	for _, deps := range [3][]string{pkg.Depends, pkg.MakeDepends, pkg.CheckDepends} {
		for _, _dep := range deps {
			dep := getNameFromDep(_dep)

			aurpkg, exists := dt.Aur[dep]
			if exists {
				_, ok := dc.Bases[aurpkg.PackageBase]
				if !ok {
					dc.Bases[aurpkg.PackageBase] = make([]*rpc.Pkg, 0)
				}
				dc.Bases[aurpkg.PackageBase] = append(dc.Bases[aurpkg.PackageBase], aurpkg)

				delete(dt.Aur, dep)
				depCatagoriesRecursive(aurpkg, dc, dt, isMake, seen)

				if !seen.get(aurpkg.PackageBase) {
					dc.Aur = append(dc.Aur, aurpkg)
					seen.set(aurpkg.PackageBase)
				}

				if isMake {
					dc.MakeOnly.set(aurpkg.Name)
				}
			}

			alpmpkg, exists := dt.Repo[dep]
			if exists {
				delete(dt.Repo, dep)
				repoDepCatagoriesRecursive(alpmpkg, dc, dt, isMake)

				if isMake {
					dc.MakeOnly.set(alpmpkg.Name())
				}

				dc.Repo = append(dc.Repo, alpmpkg)
			}

		}
		isMake = true
	}
}

func getDepTree(pkgs []string) (*depTree, error) {
	dt := makeDepTree()

	localDb, err := alpmHandle.LocalDb()
	if err != nil {
		return dt, err
	}
	syncDb, err := alpmHandle.SyncDbs()
	if err != nil {
		return dt, err
	}

	for _, pkg := range pkgs {
		//if they explicitly asked for it still look for installed pkgs
		/*installedPkg, isInstalled := localDb.PkgCache().FindSatisfier(pkg)
		if isInstalled == nil {
			dt.Repo[installedPkg.Name()] = installedPkg
			continue
		}//*/

		//check the repos for a matching dep
		repoPkg, inRepos := syncDb.FindSatisfier(pkg)
		if inRepos == nil {
			repoTreeRecursive(repoPkg, dt, localDb, syncDb)
			continue
		}

		dt.ToProcess.set(pkg)
	}

	err = depTreeRecursive(dt, localDb, syncDb, false)

	return dt, err
}

//takes a repo package
//gives all of the non installed deps
//does again on each sub dep
func repoTreeRecursive(pkg *alpm.Package, dt *depTree, localDb *alpm.Db, syncDb alpm.DbList) (err error) {
	_, exists := dt.Repo[pkg.Name()]
	if exists {
		return
	}

	dt.Repo[pkg.Name()] = pkg
	/*(*pkg).Provides().ForEach(func(dep alpm.Depend) (err error) {
		dt.Repo[dep.Name] = pkg
		return nil
	})*/

	(*pkg).Depends().ForEach(func(dep alpm.Depend) (err error) {
		_, exists := dt.Repo[dep.Name]
		if exists {
			return
		}

		_, isInstalled := localDb.PkgCache().FindSatisfier(dep.String())
		if isInstalled == nil {
			return
		}

		repoPkg, inRepos := syncDb.FindSatisfier(dep.String())
		if inRepos == nil {
			repoTreeRecursive(repoPkg, dt, localDb, syncDb)
			return
		}

		dt.Missing.set(dep.String())

		return
	})

	return
}

func depTreeRecursive(dt *depTree, localDb *alpm.Db, syncDb alpm.DbList, isMake bool) (err error) {
	if len(dt.ToProcess) == 0 {
		return
	}

	nextProcess := make(stringSet)
	currentProcess := make(stringSet)
	//strip version conditions
	for dep := range dt.ToProcess {
		currentProcess.set(getNameFromDep(dep))
	}

	//assume toprocess only contains aur stuff we have not seen
	info, err := aurInfo(currentProcess.toSlice())

	if err != nil {
		return
	}

	//cache the results
	for _, pkg := range info {
		//copying to p fixes a bug
		//would rather not copy but cant find another way to fix
		p := pkg
		dt.Aur[pkg.Name] = &p

	}

	//loop through to process and check if we now have
	//each packaged cached
	//if its not cached we assume its missing
	for pkgName := range currentProcess {
		pkg, exists := dt.Aur[pkgName]

		//did not get it in the request
		if !exists {
			dt.Missing.set(pkgName)
			continue
		}

		//for each dep and makedep
		for _, deps := range [3][]string{pkg.Depends, pkg.MakeDepends, pkg.CheckDepends} {
			for _, versionedDep := range deps {
				dep := getNameFromDep(versionedDep)

				_, exists = dt.Aur[dep]
				//we have it cached so skip
				if exists {
					continue
				}

				_, exists = dt.Repo[dep]
				//we have it cached so skip
				if exists {
					continue
				}

				_, exists = dt.Missing[dep]
				//we know it does not resolve so skip
				if exists {
					continue
				}

				//check if already installed
				_, isInstalled := localDb.PkgCache().FindSatisfier(versionedDep)
				if isInstalled == nil {
					continue
				}

				//check the repos for a matching dep
				repoPkg, inRepos := syncDb.FindSatisfier(versionedDep)
				if inRepos == nil {
					repoTreeRecursive(repoPkg, dt, localDb, syncDb)
					continue
				}

				//if all else fails add it to next search
				nextProcess.set(versionedDep)
			}
		}
	}

	dt.ToProcess = nextProcess
	depTreeRecursive(dt, localDb, syncDb, true)

	return
}

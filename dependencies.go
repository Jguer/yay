package main

import (
	"strings"

	alpm "github.com/jguer/go-alpm"
	rpc "github.com/mikkeloscar/aur"
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

//cut the version requirement from a dependency leaving just the name
func getNameFromDep(dep string) string {
	return strings.FieldsFunc(dep, func(c rune) bool {
		return c == '>' || c == '<' || c == '='
	})[0]
}

//step two of dependency reloving. We already have all the information on the
//packages we need, now it's just about ordering them correctly.
//pkgs is a list of targets, the packages we want to install, dependencies not
//included.
//For each package we want we iterate down the tree until we hit the bottom.
//This is done recursivley for each branch.
//The start of the tree is defined as the package we want.
//When we hit the bottom of the branch we know thats the first package
//we need to install so we add it to the start of the to install
//list (dc.Aur and dc.Repo).
//We work our way up until there is another branch to go down and do it all
//again.
//
//Here is a visual example
//
//      a
//     / \
//     b  c
//    / \
//   d   e
//
//we see a and it needs b and c
//we see b and it needs d and e
//we see d it needs nothing so we add d to our list and move up
//we see e it needs nothing so we add e to our list and move up
//we see c it needs nothign so we add c to our list and move up
//
//The final install order would come out as debca
//
//Theres a little more to this, handling provide, multiple packages wanting the
//same dependencies and so on this is just the basic premise.
func getDepCatagories(pkgs []string, dt *depTree) (*depCatagories, error) {
	dc := makeDependCatagories()
	seen := make(stringSet)

	for _, pkg := range dt.Aur {
		_, ok := dc.Bases[pkg.PackageBase]
		if !ok {
			dc.Bases[pkg.PackageBase] = make([]*rpc.Pkg, 0)
		}
		dc.Bases[pkg.PackageBase] = append(dc.Bases[pkg.PackageBase], pkg)
	}

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

	dupes := make(map[*alpm.Package]struct{})
	filteredRepo := make([]*alpm.Package, 0)

	for _, pkg := range dc.Repo {
		_, ok := dupes[pkg]
		if ok {
			continue
		}
		dupes[pkg] = struct{}{}
		filteredRepo = append(filteredRepo, pkg)
	}

	dc.Repo = filteredRepo

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

func depCatagoriesRecursive(_pkg *rpc.Pkg, dc *depCatagories, dt *depTree, isMake bool, seen stringSet) {
	for _, pkg := range dc.Bases[_pkg.PackageBase] {
		for _, deps := range [3][]string{pkg.Depends, pkg.MakeDepends, pkg.CheckDepends} {
			for _, _dep := range deps {
				dep := getNameFromDep(_dep)

				aurpkg, exists := dt.Aur[dep]
				if exists {
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
}

//This is step one for dependency resolving. pkgs is a slice of the packages you
//want to resolve the dependencioes for. They can be a mix of aur and repo
//dependencies. All unmet dependencies will be resolved.
//For Aur dependencies depends, makedepends and checkdepends are resolved but
//for repo packages only depends are resolved as they are pre buiild.
//The return will be split into three catagories. Repo, Aur and Missing.
//The return is in no way ordered this step is is just aimed at gathering the
//packaghes we need.
//This has been designed to make the leat amount or rpc requests as possible.
//Web requests are probably going to be the bottleneck here so minimizing them
//provides a nice spead boost.
//
//Here is a visual expample of the request system.
//Remember only unsatisfied packages are requested, if a package is already
//installed we dont bother.
//
//      a
//     / \
//     b  c
//    / \
//   d   e
//
//We see a so we send a request for a
//we see wants b and c so we send a request for b and c
//we see d and e so we send a request for d and e
//
//Thats 5 packages in 3 requests.The amount of requests needed should always be
//the same as the height of the tree.
//The example does not really do this justice, In the real world where packages
//have 10+ dependencies each this is a very nice optimization.
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

		_, isGroup := syncDb.PkgCachebyGroup(pkg)
		if isGroup == nil {
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
	(*pkg).Provides().ForEach(func(dep alpm.Depend) (err error) {
		dt.Repo[dep.Name] = pkg
		return nil
	})

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

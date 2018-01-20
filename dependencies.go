package main

import (
	"strings"

	alpm "github.com/jguer/go-alpm"
	rpc "github.com/mikkeloscar/aur"
)

type depTree struct {
	ToProcess []string
	Repo      map[string]*alpm.Package
	Aur       map[string]*rpc.Pkg
	Missing   stringSet
}

type depCatagories struct {
	Repo     []*alpm.Package
	RepoMake []*alpm.Package
	Aur      []*rpc.Pkg
	AurMake  []*rpc.Pkg
}

func makeDepTree() *depTree {
	dt := depTree{
		make([]string, 0),
		make(map[string]*alpm.Package),
		make(map[string]*rpc.Pkg),
		make(stringSet),
	}

	return &dt
}

func makeDependCatagories() *depCatagories {
	dc := depCatagories{
		make([]*alpm.Package, 0),
		make([]*alpm.Package, 0),
		make([]*rpc.Pkg, 0),
		make([]*rpc.Pkg, 0),
	}

	return &dc
}

func getNameFromDep(dep string) string {
	return strings.FieldsFunc(dep, func(c rune) bool {
		return c == '>' || c == '<' || c == '=' || c == ' '
	})[0]
}

func getDepCatagories(pkgs []string, dt *depTree) (*depCatagories, error) {
	dc := makeDependCatagories()

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
			depCatagoriesRecursive(aurpkg, dc, dt, false)
			dc.Aur = append(dc.Aur, aurpkg)
			delete(dt.Aur, dep)
		}
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
				dc.RepoMake = append(dc.RepoMake, alpmpkg)
			} else {
				dc.Repo = append(dc.Repo, alpmpkg)
			}

		}

		return nil
	})
}

func depCatagoriesRecursive(pkg *rpc.Pkg, dc *depCatagories, dt *depTree, isMake bool) {
	for _, deps := range [2][]string{pkg.Depends, pkg.MakeDepends} {
		for _, _dep := range deps {
			dep := getNameFromDep(_dep)

			aurpkg, exists := dt.Aur[dep]
			if exists {
				delete(dt.Aur, dep)
				depCatagoriesRecursive(aurpkg, dc, dt, isMake)

				if isMake {
					dc.AurMake = append(dc.AurMake, aurpkg)
				} else {
					dc.Aur = append(dc.Aur, aurpkg)
				}

			}

			alpmpkg, exists := dt.Repo[dep]
			if exists {
				delete(dt.Repo, dep)
				repoDepCatagoriesRecursive(alpmpkg, dc, dt, isMake)

				if isMake {
					dc.RepoMake = append(dc.RepoMake, alpmpkg)
				} else {
					dc.Repo = append(dc.Repo, alpmpkg)
				}

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

		dt.ToProcess = append(dt.ToProcess, pkg)
	}

	if len(dt.ToProcess) > 0 {
		err = depTreeRecursive(dt, localDb, syncDb, false)
	}

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
		} else {
			dt.Missing.set(dep.String())
		}

		return
	})

	return
}

func depTreeRecursive(dt *depTree, localDb *alpm.Db, syncDb alpm.DbList, isMake bool) (err error) {
	nextProcess := make([]string, 0)
	currentProcess := make([]string, 0, len(dt.ToProcess))

	//strip version conditions
	for _, dep := range dt.ToProcess {
		currentProcess = append(currentProcess, getNameFromDep(dep))
	}

	//assume toprocess only contains aur stuff we have not seen
	info, err := rpc.Info(currentProcess)
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
	for k, pkgName := range currentProcess {
		pkg, exists := dt.Aur[pkgName]

		//didnt get it in the request
		if !exists {
			dt.Missing.set(dt.ToProcess[k])
			continue
		}

		//for reach dep and makedep
		for _, deps := range [2][]string{pkg.Depends, pkg.MakeDepends} {
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
				//we know it doesnt resolve so skip
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

				//if all else failes add it to next search
				nextProcess = append(nextProcess, versionedDep)
			}
		}
	}

	dt.ToProcess = nextProcess
	depTreeRecursive(dt, localDb, syncDb, true)

	return
}

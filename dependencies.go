package main

import (
	"fmt"
	"strings"

	alpm "github.com/jguer/go-alpm"
	rpc "github.com/mikkeloscar/aur"
	gopkg "github.com/mikkeloscar/gopkgbuild"
)

type depTree struct {
	ToProcess stringSet
	Repo      map[string]*alpm.Package
	Aur       map[string]*rpc.Pkg
	Missing   stringSet
	Groups    stringSet
	Provides  map[string]string
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
		make(stringSet),
		make(map[string]string),
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

// Cut the version requirement from a dependency leaving just the name.
func splitNameFromDep(dep string) (string, string) {
	split := strings.FieldsFunc(dep, func(c rune) bool {
		return c == '>' || c == '<' || c == '='
	})

	if len(split) == 1 {
		return split[0], ""
	}

	return split[0], split[1]
}

//split apart db/package to db and package
func splitDbFromName(pkg string) (string, string) {
	split := strings.SplitN(pkg, "/", 2)

	if len(split) == 2 {
		return split[0], split[1]
	}
	return "", split[0]
}

func isDevelName(name string) bool {
	for _, suffix := range []string{"git", "svn", "hg", "bzr", "nightly"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}

	return strings.Contains(name, "-always-")
}

func getBases(pkgs map[string]*rpc.Pkg) map[string][]*rpc.Pkg {
	bases := make(map[string][]*rpc.Pkg)

nextpkg:
	for _, pkg := range pkgs {
		for _, base := range bases[pkg.PackageBase] {
			if base == pkg {
				continue nextpkg
			}
		}

		_, ok := bases[pkg.PackageBase]
		if !ok {
			bases[pkg.PackageBase] = make([]*rpc.Pkg, 0)
		}
		bases[pkg.PackageBase] = append(bases[pkg.PackageBase], pkg)
	}

	return bases
}

func aurFindProvider(name string, dt *depTree) (string, *rpc.Pkg) {
	dep, _ := splitNameFromDep(name)
	aurpkg, exists := dt.Aur[dep]

	if exists {
		return dep, aurpkg
	}

	dep, exists = dt.Provides[dep]
	if exists {
		aurpkg, exists = dt.Aur[dep]
		if exists {
			return dep, aurpkg
		}
	}

	return "", nil

}

func repoFindProvider(name string, dt *depTree) (string, *alpm.Package) {
	dep, _ := splitNameFromDep(name)
	alpmpkg, exists := dt.Repo[dep]

	if exists {
		return dep, alpmpkg
	}

	dep, exists = dt.Provides[dep]
	if exists {
		alpmpkg, exists = dt.Repo[dep]
		if exists {
			return dep, alpmpkg
		}
	}

	return "", nil

}

// Step two of dependency resolving. We already have all the information on the
// packages we need, now it's just about ordering them correctly.
// pkgs is a list of targets, the packages we want to install. Dependencies are
// not included.
// For each package we want we iterate down the tree until we hit the bottom.
// This is done recursively for each branch.
// The start of the tree is defined as the package we want.
// When we hit the bottom of the branch we know thats the first package
// we need to install so we add it to the start of the to install
// list (dc.Aur and dc.Repo).
// We work our way up until there is another branch to go down and do it all
// again.
//
// Here is a visual example:
//
//       a
//      / \
//      b  c
//     / \
//    d   e
//
// We see a and it needs b and c
// We see b and it needs d and e
// We see d - it needs nothing so we add d to our list and move up
// We see e - it needs nothing so we add e to our list and move up
// We see c - it needs nothing so we add c to our list and move up
//
// The final install order would come out as debca
//
// There is a little more to this, handling provides, multiple packages wanting the
// same dependencies, etc. This is just the basic premise.
func getDepCatagories(pkgs []string, dt *depTree) (*depCatagories, error) {
	dc := makeDependCatagories()
	seen := make(stringSet)

	dc.Bases = getBases(dt.Aur)

	for _, pkg := range pkgs {
		dep, alpmpkg := repoFindProvider(pkg, dt)
		if alpmpkg != nil {
			repoDepCatagoriesRecursive(alpmpkg, dc, dt, false)
			dc.Repo = append(dc.Repo, alpmpkg)
			delete(dt.Repo, dep)
		}

		dep, aurpkg := aurFindProvider(pkg, dt)
		if aurpkg != nil {
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

	for _, pkg := range pkgs {
		dc.MakeOnly.remove(pkg)
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
		dep, alpmpkg := repoFindProvider(_dep.Name, dt)
		if alpmpkg != nil {
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
			for _, pkg := range deps {
				dep, aurpkg := aurFindProvider(pkg, dt)
				if aurpkg != nil {
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

				dep, alpmpkg := repoFindProvider(pkg, dt)
				if alpmpkg != nil {
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

// This is step one for dependency resolving. pkgs is a slice of the packages you
// want to resolve the dependencies for. They can be a mix of aur and repo
// dependencies. All unmet dependencies will be resolved.
//
// For Aur dependencies depends, makedepends and checkdepends are resolved but
// for repo packages only depends are resolved as they are prebuilt.
// The return will be split into three catagories: Repo, Aur and Missing.
// The return is in no way ordered. This step is is just aimed at gathering the
// packages we need.
//
// This has been designed to make the least amount of rpc requests as possible.
// Web requests are probably going to be the bottleneck here so minimizing them
// provides a nice speed boost.
//
// Here is a visual expample of the request system.
// Remember only unsatisfied packages are requested, if a package is already
// installed we dont bother.
//
//      a
//     / \
//     b  c
//    / \
//   d   e
//
// We see a so we send a request for a
// We see a wants b and c so we send a request for b and c
// We see d and e so we send a request for d and e
//
// Thats 5 packages in 3 requests. The amount of requests needed should always be
// the same as the height of the tree.
// The example does not really do this justice, In the real world where packages
// have 10+ dependencies each this is a very nice optimization.
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
		db, name := splitDbFromName(pkg)
		var foundPkg *alpm.Package
		var singleDb *alpm.Db

		if db == "aur" {
			dt.ToProcess.set(name)
			continue
		}

		// Check the repos for a matching dep
		if db != "" {
			singleDb, err = alpmHandle.SyncDbByName(db)
			if err != nil {
				return dt, err
			}
			foundPkg, err = singleDb.PkgCache().FindSatisfier(name)
		} else {
			foundPkg, err = syncDb.FindSatisfier(name)
		}

		if err == nil {
			repoTreeRecursive(foundPkg, dt, localDb, syncDb)
			continue
		} else {
			//would be better to check the groups from singleDb if
			//the user specified a db but theres no easy way to do
			//it without making alpm_lists so dont bother for now
			//db/group is probably a rare use case
			_, err := syncDb.PkgCachebyGroup(name)

			if err == nil {
				dt.Groups.set(pkg)
				continue
			}
		}

		if db == "" {
			dt.ToProcess.set(name)
		} else {
			dt.Missing.set(pkg)
		}
	}

	if len(dt.ToProcess) > 0 {
		fmt.Println(bold(cyan("::") + " Querying AUR..."))
	}

	err = depTreeRecursive(dt, localDb, syncDb, false)
	if err != nil {
		return dt, err
	}

	if !cmdArgs.existsArg("d", "nodeps") {
		err = checkVersions(dt)
	}

	return dt, err
}

// Takes a repo package,
// gives all of the non installed deps,
// repeats on each sub dep.
func repoTreeRecursive(pkg *alpm.Package, dt *depTree, localDb *alpm.Db, syncDb alpm.DbList) (err error) {
	_, exists := dt.Repo[pkg.Name()]
	if exists {
		return
	}

	_, exists = dt.Provides[pkg.Name()]
	if exists {
		return
	}

	dt.Repo[pkg.Name()] = pkg
	(*pkg).Provides().ForEach(func(dep alpm.Depend) (err error) {
		dt.Provides[dep.Name] = pkg.Name()
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
	// Strip version conditions
	for _dep := range dt.ToProcess {
		dep, _ := splitNameFromDep(_dep)
		currentProcess.set(dep)
	}

	// Assume toprocess only contains aur stuff we have not seen
	info, err := aurInfo(currentProcess.toSlice())

	if err != nil {
		return
	}

	// Cache the results
	for _, pkg := range info {
		dt.Aur[pkg.Name] = pkg

		for _, provide := range pkg.Provides {
			name, _ := splitNameFromDep(provide)
			dt.Provides[name] = pkg.Name
		}
	}

	// Loop through to process and check if we now have
	// each packaged cached.
	// If not cached, we assume it is missing.
	for pkgName := range currentProcess {
		pkg, exists := dt.Aur[pkgName]

		// Did not get it in the request.
		if !exists {
			dt.Missing.set(pkgName)
			continue
		}

		// for each dep and makedep
		for _, deps := range [3][]string{pkg.Depends, pkg.MakeDepends, pkg.CheckDepends} {
			for _, versionedDep := range deps {
				dep, _ := splitNameFromDep(versionedDep)

				_, exists = dt.Aur[dep]
				// We have it cached so skip.
				if exists {
					continue
				}

				_, exists = dt.Provides[dep]
				// We have it cached so skip.
				if exists {
					continue
				}

				_, exists = dt.Repo[dep]
				// We have it cached so skip.
				if exists {
					continue
				}

				_, exists = dt.Missing[dep]
				// We know it does not resolve so skip.
				if exists {
					continue
				}

				// Check if already installed.
				_, isInstalled := localDb.PkgCache().FindSatisfier(versionedDep)
				if isInstalled == nil && config.ReBuild != "tree" {
					continue
				}

				// Check the repos for a matching dep.
				repoPkg, inRepos := syncDb.FindSatisfier(versionedDep)
				if inRepos == nil {
					if isInstalled == nil && config.ReBuild == "tree" {
						continue
					}

					repoTreeRecursive(repoPkg, dt, localDb, syncDb)
					continue
				}

				// If all else fails add it to next search.
				nextProcess.set(versionedDep)
			}
		}
	}

	dt.ToProcess = nextProcess
	depTreeRecursive(dt, localDb, syncDb, true)

	return
}

func checkVersions(dt *depTree) error {
	has := make(map[string][]string)
	allDeps := make([]*gopkg.Dependency, 0)

	localDb, err := alpmHandle.LocalDb()
	if err != nil {
		return err
	}

	for _, pkg := range dt.Aur {
		for _, deps := range [3][]string{pkg.Depends, pkg.MakeDepends, pkg.CheckDepends} {
			for _, dep := range deps {
				_, _dep := splitNameFromDep(dep)
				if _dep != "" {
					deps, _ := gopkg.ParseDeps([]string{dep})
					if deps[0] != nil {
						allDeps = append(allDeps, deps[0])
					}
				}
			}
		}

		addMapStringSlice(has, pkg.Name, pkg.Version)

		if !isDevelName(pkg.Name) {
			for _, name := range pkg.Provides {
				_name, _ver := splitNameFromDep(name)
				if _ver != "" {
					addMapStringSlice(has, _name, _ver)
				} else {
					delete(has, _name)
				}
			}
		}
	}

	for _, pkg := range dt.Repo {
		pkg.Depends().ForEach(func(dep alpm.Depend) error {
			if dep.Mod != alpm.DepModAny {
				deps, _ := gopkg.ParseDeps([]string{dep.String()})
				if deps[0] != nil {
					allDeps = append(allDeps, deps[0])
				}
			}
			return nil
		})

		addMapStringSlice(has, pkg.Name(), pkg.Version())

		pkg.Provides().ForEach(func(dep alpm.Depend) error {
			if dep.Mod != alpm.DepModAny {
				addMapStringSlice(has, dep.Name, dep.Version)
			} else {
				delete(has, dep.Name)
			}

			return nil
		})

	}

	localDb.PkgCache().ForEach(func(pkg alpm.Package) error {
		pkg.Provides().ForEach(func(dep alpm.Depend) error {
			if dep.Mod != alpm.DepModAny {
				addMapStringSlice(has, dep.Name, dep.Version)
			} else {
				delete(has, dep.Name)
			}

			return nil
		})

		return nil
	})

	for _, dep := range allDeps {
		satisfied := false
		verStrs, ok := has[dep.Name]
		if !ok {
			continue
		}

		for _, verStr := range verStrs {
			version, err := gopkg.NewCompleteVersion(verStr)
			if err != nil {
				return err
			}

			if version.Satisfies(dep) {
				satisfied = true
				break
			}
		}

		if !satisfied {
			dt.Missing.set(dep.String())
		}
	}

	return nil
}

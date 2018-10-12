package main

import (
	"fmt"
	"strings"
	"sync"

	alpm "github.com/jguer/go-alpm"
	rpc "github.com/mikkeloscar/aur"
)

func (ds *depSolver) _checkMissing(dep string, stack []string, missing *missing) {
	if missing.Good.get(dep) {
		return
	}

	if trees, ok := missing.Missing[dep]; ok {
		for _, tree := range trees {
			if stringSliceEqual(tree, stack) {
				return
			}
		}
		missing.Missing[dep] = append(missing.Missing[dep], stack)
		return
	}

	aurPkg := ds.findSatisfierAur(dep)
	if aurPkg != nil {
		missing.Good.set(dep)
		for _, deps := range [3][]string{aurPkg.Depends, aurPkg.MakeDepends, aurPkg.CheckDepends} {
			for _, aurDep := range deps {
				if _, err := ds.LocalDb.PkgCache().FindSatisfier(aurDep); err == nil {
					missing.Good.set(aurDep)
					continue
				}

				ds._checkMissing(aurDep, append(stack, aurPkg.Name), missing)
			}
		}

		return
	}

	repoPkg := ds.findSatisfierRepo(dep)
	if repoPkg != nil {
		missing.Good.set(dep)
		repoPkg.Depends().ForEach(func(repoDep alpm.Depend) error {
			if _, err := ds.LocalDb.PkgCache().FindSatisfier(repoDep.String()); err == nil {
				missing.Good.set(repoDep.String())
				return nil
			}

			ds._checkMissing(repoDep.String(), append(stack, repoPkg.Name()), missing)
			return nil
		})

		return
	}

	missing.Missing[dep] = [][]string{stack}
}

func (ds *depSolver) CheckMissing() error {
	missing := &missing{
		make(stringSet),
		make(map[string][][]string),
	}

	for _, target := range ds.Targets {
		ds._checkMissing(target.DepString(), make([]string, 0), missing)
	}

	if len(missing.Missing) == 0 {
		return nil
	}

	fmt.Println(bold(red(arrow+" Error: ")) + "Could not find all required packages:")
	for dep, trees := range missing.Missing {
		for _, tree := range trees {

			fmt.Print("    ", cyan(dep))

			if len(tree) == 0 {
				fmt.Print(" (Target")
			} else {
				fmt.Print(" (Wanted by: ")
				for n := 0; n < len(tree)-1; n++ {
					fmt.Print(cyan(tree[n]), " -> ")
				}
				fmt.Print(cyan(tree[len(tree)-1]))
			}

			fmt.Println(")")
		}
	}

	return fmt.Errorf("")
}

func (ds *depSolver) checkForwardConflict(name string, conflict string, conflicts mapStringSet) {
	ds.LocalDb.PkgCache().ForEach(func(pkg alpm.Package) error {
		if pkg.Name() == name || ds.hasPackage(pkg.Name()) {
			return nil
		}

		if satisfiesRepo(conflict, &pkg) {
			n := pkg.Name()
			conflicts.Add(name, n)
		}

		return nil
	})
}

func (ds *depSolver) checkReverseConflict(name string, conflict string, conflicts mapStringSet) {
	for _, base := range ds.Aur {
		for _, pkg := range base {
			if pkg.Name == name {
				continue
			}

			if satisfiesAur(conflict, pkg) {
				conflicts.Add(pkg.Name, name)
			}
		}
	}

	for _, pkg := range ds.Repo {
		if pkg.Name() == name {
			continue
		}

		if satisfiesRepo(conflict, pkg) {
			conflicts.Add(pkg.Name(), name)
		}
	}
}

func (ds *depSolver) checkForwardConflicts(conflicts mapStringSet) {
	for _, base := range ds.Aur {
		for _, pkg := range base {
			for _, conflict := range pkg.Conflicts {
				ds.checkForwardConflict(pkg.Name, conflict, conflicts)
			}
		}
	}

	for _, pkg := range ds.Repo {
		pkg.Conflicts().ForEach(func(conflict alpm.Depend) error {
			ds.checkForwardConflict(pkg.Name(), conflict.String(), conflicts)
			return nil
		})
	}
}

func (ds *depSolver) checkReverseConflicts(conflicts mapStringSet) {
	ds.LocalDb.PkgCache().ForEach(func(pkg alpm.Package) error {
		if ds.hasPackage(pkg.Name()) {
			return nil
		}

		pkg.Conflicts().ForEach(func(conflict alpm.Depend) error {
			ds.checkReverseConflict(pkg.Name(), conflict.String(), conflicts)
			return nil
		})

		return nil
	})
}

func (ds *depSolver) checkInnerRepoConflicts(conflicts mapStringSet) {
	for _, pkg := range ds.Repo {
		pkg.Conflicts().ForEach(func(conflict alpm.Depend) error {
			for _, innerpkg := range ds.Repo {
				if pkg.Name() != innerpkg.Name() && satisfiesRepo(conflict.String(), innerpkg) {
					conflicts.Add(pkg.Name(), innerpkg.Name())
				}
			}

			return nil
		})
	}
}

func (ds *depSolver) checkInnerConflicts(conflicts mapStringSet) {
	removed := make(stringSet)
	//ds.checkInnerConflictRepoAur(conflicts)

	for current, currbase := range ds.Aur {
		for _, pkg := range currbase {
			ds.checkInnerConflict(pkg, ds.Aur[:current], removed, conflicts)
		}
	}
}

// Check if anything conflicts with currpkg
// If so add the conflict with currpkg being removed by the conflicting pkg
func (ds *depSolver) checkInnerConflict(currpkg *rpc.Pkg, aur []Base, removed stringSet, conflicts mapStringSet) {
	for _, base := range aur {
		for _, pkg := range base {
			for _, conflict := range pkg.Conflicts {
				if !removed.get(pkg.Name) && satisfiesAur(conflict, currpkg) {
					addInnerConflict(pkg.Name, currpkg.Name, removed, conflicts)
				}
			}
		}
	}
	for _, pkg := range ds.Repo {
		pkg.Conflicts().ForEach(func(conflict alpm.Depend) error {
			if !removed.get(pkg.Name()) && satisfiesAur(conflict.String(), currpkg) {
				addInnerConflict(pkg.Name(), currpkg.Name, removed, conflicts)
			}
			return nil
		})
	}

	for _, conflict := range currpkg.Conflicts {
		for _, base := range aur {
			for _, pkg := range base {
				if !removed.get(pkg.Name) && satisfiesAur(conflict, pkg) {
					addInnerConflict(pkg.Name, currpkg.Name, removed, conflicts)
				}
			}
		}
		for _, pkg := range ds.Repo {
			if !removed.get(pkg.Name()) && satisfiesRepo(conflict, pkg) {
				addInnerConflict(pkg.Name(), currpkg.Name, removed, conflicts)
			}
		}
	}
}

func addInnerConflict(toRemove string, removedBy string, removed stringSet, conflicts mapStringSet) {
	conflicts.Add(removedBy, toRemove)
	removed.set(toRemove)
}

func (ds *depSolver) CheckConflicts() (mapStringSet, error) {
	var wg sync.WaitGroup
	innerConflicts := make(mapStringSet)
	conflicts := make(mapStringSet)
	repoConflicts := make(mapStringSet)
	wg.Add(3)

	fmt.Println(bold(cyan("::") + bold(" Checking for conflicts...")))
	go func() {
		ds.checkForwardConflicts(conflicts)
		ds.checkReverseConflicts(conflicts)
		wg.Done()
	}()

	fmt.Println(bold(cyan("::") + bold(" Checking for inner conflicts...")))
	go func() {
		ds.checkInnerConflicts(innerConflicts)
		wg.Done()
	}()
	go func() {
		ds.checkInnerRepoConflicts(repoConflicts)
		wg.Done()
	}()

	wg.Wait()

	formatConflicts := func(conflicts mapStringSet, inner bool, s string) {
		if len(conflicts) != 0 {
			fmt.Println()
			if inner {
				fmt.Println(bold(red(arrow)), bold("Inner conflicts found:"))
			} else {
				fmt.Println(bold(red(arrow)), bold("Package conflicts found:"))
			}

			for name, pkgs := range conflicts {
				str := fmt.Sprintf(s, cyan(name))
				for pkg := range pkgs {
					str += " " + cyan(pkg) + ","
				}
				str = strings.TrimSuffix(str, ",")

				fmt.Println(str)
			}
		}
	}

	repoStr := red(bold(smallArrow)) + " %s Conflicts with:"
	formatConflicts(repoConflicts, true, repoStr)

	if len(repoConflicts) > 0 {
		return nil, fmt.Errorf("Unavoidable conflicts, aborting")
	}

	str := red(bold(smallArrow)) + " Installing %s will remove:"
	formatConflicts(conflicts, false, str)
	formatConflicts(innerConflicts, true, str)

	for name, c := range innerConflicts {
		for cs, _ := range c {
			conflicts.Add(name, cs)
		}
	}

	if len(conflicts) > 0 {
		if !config.UseAsk {
			if config.NoConfirm {
				return nil, fmt.Errorf("Package conflicts can not be resolved with noconfirm, aborting")
			}

			fmt.Println()
			fmt.Println(bold(red(arrow)), bold("Conflicting packages will have to be confirmed manually"))
			fmt.Println()
		}
	}

	return conflicts, nil
}

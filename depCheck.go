package main

import (
	"fmt"
	"os"
	"strings"
	"sync"

	alpm "github.com/jguer/go-alpm"
)

func (dp *depPool) checkInnerConflict(name string, conflict string, conflicts mapStringSet) {
	for _, pkg := range dp.Aur {
		if pkg.Name == name {
			continue
		}

		if satisfiesAur(conflict, pkg) {
			conflicts.Add(name, pkg.Name)
		}
	}

	for _, pkg := range dp.Repo {
		if pkg.Name() == name {
			continue
		}

		if satisfiesRepo(conflict, pkg) {
			conflicts.Add(name, pkg.Name())
		}
	}
}

func (dp *depPool) checkForwardConflict(name string, conflict string, conflicts mapStringSet) {
	dp.LocalDB.PkgCache().ForEach(func(pkg alpm.Package) error {
		if pkg.Name() == name || dp.hasPackage(pkg.Name()) {
			return nil
		}

		if satisfiesRepo(conflict, &pkg) {
			n := pkg.Name()
			if n != conflict {
				n += " (" + conflict + ")"
			}
			conflicts.Add(name, n)
		}

		return nil
	})
}

func (dp *depPool) checkReverseConflict(name string, conflict string, conflicts mapStringSet) {
	for _, pkg := range dp.Aur {
		if pkg.Name == name {
			continue
		}

		if satisfiesAur(conflict, pkg) {
			if name != conflict {
				name += " (" + conflict + ")"
			}

			conflicts.Add(pkg.Name, name)
		}
	}

	for _, pkg := range dp.Repo {
		if pkg.Name() == name {
			continue
		}

		if satisfiesRepo(conflict, pkg) {
			if name != conflict {
				name += " (" + conflict + ")"
			}

			conflicts.Add(pkg.Name(), name)
		}
	}
}

func (dp *depPool) checkInnerConflicts(conflicts mapStringSet) {
	for _, pkg := range dp.Aur {
		for _, conflict := range pkg.Conflicts {
			dp.checkInnerConflict(pkg.Name, conflict, conflicts)
		}
	}

	for _, pkg := range dp.Repo {
		pkg.Conflicts().ForEach(func(conflict alpm.Depend) error {
			dp.checkInnerConflict(pkg.Name(), conflict.String(), conflicts)
			return nil
		})
	}
}

func (dp *depPool) checkForwardConflicts(conflicts mapStringSet) {
	for _, pkg := range dp.Aur {
		for _, conflict := range pkg.Conflicts {
			dp.checkForwardConflict(pkg.Name, conflict, conflicts)
		}
	}

	for _, pkg := range dp.Repo {
		pkg.Conflicts().ForEach(func(conflict alpm.Depend) error {
			dp.checkForwardConflict(pkg.Name(), conflict.String(), conflicts)
			return nil
		})
	}
}

func (dp *depPool) checkReverseConflicts(conflicts mapStringSet) {
	dp.LocalDB.PkgCache().ForEach(func(pkg alpm.Package) error {
		if dp.hasPackage(pkg.Name()) {
			return nil
		}

		pkg.Conflicts().ForEach(func(conflict alpm.Depend) error {
			dp.checkReverseConflict(pkg.Name(), conflict.String(), conflicts)
			return nil
		})

		return nil
	})
}

func (dp *depPool) CheckConflicts() (mapStringSet, error) {
	var wg sync.WaitGroup
	innerConflicts := make(mapStringSet)
	conflicts := make(mapStringSet)
	wg.Add(2)

	fmt.Println(bold(cyan("::") + bold(" Checking for conflicts...")))
	go func() {
		dp.checkForwardConflicts(conflicts)
		dp.checkReverseConflicts(conflicts)
		wg.Done()
	}()

	fmt.Println(bold(cyan("::") + bold(" Checking for inner conflicts...")))
	go func() {
		dp.checkInnerConflicts(innerConflicts)
		wg.Done()
	}()

	wg.Wait()

	if len(innerConflicts) != 0 {
		fmt.Println()
		fmt.Println(bold(red(arrow)), bold("Inner conflicts found:"))

		for name, pkgs := range innerConflicts {
			str := red(bold(smallArrow)) + " " + name + ":"
			for pkg := range pkgs {
				str += " " + cyan(pkg) + ","
			}
			str = strings.TrimSuffix(str, ",")

			fmt.Println(str)
		}

	}

	if len(conflicts) != 0 {
		fmt.Println()
		fmt.Println(bold(red(arrow)), bold("Package conflicts found:"))

		for name, pkgs := range conflicts {
			str := red(bold(smallArrow)) + " Installing " + cyan(name) + " will remove:"
			for pkg := range pkgs {
				str += " " + cyan(pkg) + ","
			}
			str = strings.TrimSuffix(str, ",")

			fmt.Println(str)
		}

	}

	// Add the inner conflicts to the conflicts
	// These are used to decide what to pass --ask to (if set) or don't pass --noconfirm to
	// As we have no idea what the order is yet we add every inner conflict to the slice
	for name, pkgs := range innerConflicts {
		conflicts[name] = make(stringSet)
		for pkg := range pkgs {
			conflicts[pkg] = make(stringSet)
		}
	}

	if len(conflicts) > 0 {
		if !config.UseAsk {
			if config.NoConfirm {
				return nil, fmt.Errorf("Package conflicts can not be resolved with noconfirm, aborting")
			}

			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, bold(red(arrow)), bold("Conflicting packages will have to be confirmed manually"))
			fmt.Fprintln(os.Stderr)
		}
	}

	return conflicts, nil
}

type missing struct {
	Good    stringSet
	Missing map[string][][]string
}

func (dp *depPool) _checkMissing(dep string, stack []string, missing *missing) {
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

	aurPkg := dp.findSatisfierAur(dep)
	if aurPkg != nil {
		missing.Good.set(dep)
		for _, deps := range [3][]string{aurPkg.Depends, aurPkg.MakeDepends, aurPkg.CheckDepends} {
			for _, aurDep := range deps {
				if _, err := dp.LocalDB.PkgCache().FindSatisfier(aurDep); err == nil {
					missing.Good.set(aurDep)
					continue
				}

				dp._checkMissing(aurDep, append(stack, aurPkg.Name), missing)
			}
		}

		return
	}

	repoPkg := dp.findSatisfierRepo(dep)
	if repoPkg != nil {
		missing.Good.set(dep)
		repoPkg.Depends().ForEach(func(repoDep alpm.Depend) error {
			if _, err := dp.LocalDB.PkgCache().FindSatisfier(repoDep.String()); err == nil {
				missing.Good.set(repoDep.String())
				return nil
			}

			dp._checkMissing(repoDep.String(), append(stack, repoPkg.Name()), missing)
			return nil
		})

		return
	}

	missing.Missing[dep] = [][]string{stack}
}

func (dp *depPool) CheckMissing() error {
	missing := &missing{
		make(stringSet),
		make(map[string][][]string),
	}

	for _, target := range dp.Targets {
		dp._checkMissing(target.DepString(), make([]string, 0), missing)
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

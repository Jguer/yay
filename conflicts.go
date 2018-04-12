package main

import (
	"fmt"
	"strings"
	"sync"

	alpm "github.com/jguer/go-alpm"
	gopkg "github.com/mikkeloscar/gopkgbuild"
)

// Checks a single conflict against every other to be installed package's
// name and its provides.
func checkInnerConflict(name string, conflict string, conflicts map[string]stringSet, dc *depCatagories) {
	deps, err := gopkg.ParseDeps([]string{conflict})
	if err != nil {
		return
	}
	dep := deps[0]

	for _, pkg := range dc.Aur {
		if name == pkg.Name {
			continue
		}

		version, err := gopkg.NewCompleteVersion(pkg.Version)
		if err != nil {
			return
		}
		if dep.Name == pkg.Name && version.Satisfies(dep) {
			addMapStringSet(conflicts, name, pkg.Name)
			continue
		}

		for _, provide := range pkg.Provides {
			// Provides are not versioned unless explicitly defined as
			// such. If a conflict is versioned but a provide is
			// not it can not conflict.
			if (dep.MaxVer != nil || dep.MinVer != nil) && !strings.ContainsAny(provide, "><=") {
				continue
			}

			var version *gopkg.CompleteVersion
			var err error

			pname, pversion := splitNameFromDep(provide)

			if dep.Name != pname {
				continue
			}

			if pversion != "" {
				version, err = gopkg.NewCompleteVersion(provide)
				if err != nil {
					return
				}
			}

			if version != nil && version.Satisfies(dep) {
				addMapStringSet(conflicts, name, pkg.Name)
				break
			}

		}
	}

	for _, pkg := range dc.Repo {
		if name == pkg.Name() {
			continue
		}

		version, err := gopkg.NewCompleteVersion(pkg.Version())
		if err != nil {
			return
		}

		if dep.Name == pkg.Name() && version.Satisfies(dep) {
			addMapStringSet(conflicts, name, pkg.Name())
			continue
		}

		pkg.Provides().ForEach(func(provide alpm.Depend) error {
			// Provides are not versioned unless explicitly defined as
			// such. If a conflict is versioned but a provide is
			// not it can not conflict.
			if (dep.MaxVer != nil || dep.MinVer != nil) && provide.Mod == alpm.DepModAny {
				return nil
			}

			if dep.Name != pkg.Name() {
				return nil
			}

			if provide.Mod == alpm.DepModAny {
				addMapStringSet(conflicts, name, pkg.Name())
				return fmt.Errorf("")
			}

			version, err := gopkg.NewCompleteVersion(provide.Version)
			if err != nil {
				return nil
			}

			if version.Satisfies(dep) {
				addMapStringSet(conflicts, name, pkg.Name())
				return fmt.Errorf("")
			}

			return nil
		})
	}
}

// Checks every to be installed package's conflicts against every other to be
// installed package and its provides.
func checkForInnerConflicts(dc *depCatagories) map[string]stringSet {
	conflicts := make(map[string]stringSet)

	for _, pkg := range dc.Aur {
		for _, cpkg := range pkg.Conflicts {
			checkInnerConflict(pkg.Name, cpkg, conflicts, dc)
		}
	}

	for _, pkg := range dc.Repo {
		pkg.Conflicts().ForEach(func(conflict alpm.Depend) error {
			checkInnerConflict(pkg.Name(), conflict.String(), conflicts, dc)
			return nil
		})
	}

	return conflicts
}

// Checks a provide or packagename from a to be installed package
// against every already installed package's conflicts
func checkReverseConflict(name string, provide string, conflicts map[string]stringSet) error {
	var version *gopkg.CompleteVersion
	var err error

	localDb, err := alpmHandle.LocalDb()
	if err != nil {
		return err
	}

	pname, pversion := splitNameFromDep(provide)
	if pversion != "" {
		version, err = gopkg.NewCompleteVersion(pversion)
		if err != nil {
			return nil
		}
	}

	localDb.PkgCache().ForEach(func(pkg alpm.Package) error {
		if name == pkg.Name() {
			return nil
		}

		pkg.Conflicts().ForEach(func(conflict alpm.Depend) error {
			deps, err := gopkg.ParseDeps([]string{conflict.String()})
			if err != nil {
				return nil
			}

			dep := deps[0]
			// Provides are not versioned unless explicitly defined as
			// such. If a conflict is versioned but a provide is
			// not it can not conflict.
			if (dep.MaxVer != nil || dep.MinVer != nil) && version == nil {
				return nil
			}

			if dep.Name != pname {
				return nil
			}

			if version == nil || version.Satisfies(dep) {
				// Todo
				addMapStringSet(conflicts, name, pkg.Name()+" ("+provide+")")
				return fmt.Errorf("")
			}

			return nil
		})

		return nil
	})

	return nil
}

// Checks the conflict of a to be installed package against the package name and
// provides of every installed package.
func checkConflict(name string, conflict string, conflicts map[string]stringSet) error {
	localDb, err := alpmHandle.LocalDb()
	if err != nil {
		return err
	}

	deps, err := gopkg.ParseDeps([]string{conflict})
	if err != nil {
		return nil
	}

	dep := deps[0]

	localDb.PkgCache().ForEach(func(pkg alpm.Package) error {
		if name == pkg.Name() {
			return nil
		}

		version, err := gopkg.NewCompleteVersion(pkg.Version())
		if err != nil {
			return nil
		}

		if dep.Name == pkg.Name() && version.Satisfies(dep) {
			addMapStringSet(conflicts, name, pkg.Name())
			return nil
		}

		pkg.Provides().ForEach(func(provide alpm.Depend) error {
			if dep.Name != provide.Name {
				return nil
			}

			// Provides arent version unless explicitly defined as
			// such. If a conflict is versioned but a provide is
			// not it can not conflict.
			if (dep.MaxVer != nil || dep.MinVer != nil) && provide.Mod == alpm.DepModAny {
				return nil
			}

			if provide.Mod == alpm.DepModAny {
				addMapStringSet(conflicts, name, pkg.Name()+" ("+provide.Name+")")
				return fmt.Errorf("")
			}

			version, err := gopkg.NewCompleteVersion(provide.Version)
			if err != nil {
				return nil
			}

			if version.Satisfies(dep) {
				addMapStringSet(conflicts, name, pkg.Name()+" ("+provide.Name+")")
				return fmt.Errorf("")
			}

			return nil
		})

		return nil
	})

	return nil
}

// Checks every to be installed package's conflicts against the names and
// provides of every already installed package and checks every to be installed
// package's name and provides against every already installed package.
func checkForConflicts(dc *depCatagories) (map[string]stringSet, error) {
	conflicts := make(map[string]stringSet)

	for _, pkg := range dc.Aur {
		for _, cpkg := range pkg.Conflicts {
			checkConflict(pkg.Name, cpkg, conflicts)
		}
	}

	for _, pkg := range dc.Repo {
		pkg.Conflicts().ForEach(func(conflict alpm.Depend) error {
			checkConflict(pkg.Name(), conflict.String(), conflicts)
			return nil
		})
	}

	for _, pkg := range dc.Aur {
		checkReverseConflict(pkg.Name, pkg.Name, conflicts)
		for _, ppkg := range pkg.Provides {
			checkReverseConflict(pkg.Name, ppkg, conflicts)
		}
	}

	for _, pkg := range dc.Repo {
		checkReverseConflict(pkg.Name(), pkg.Name(), conflicts)
		pkg.Provides().ForEach(func(provide alpm.Depend) error {
			checkReverseConflict(pkg.Name(), provide.String(), conflicts)
			return nil
		})
	}

	return conflicts, nil
}

// Combiles checkForConflicts() and checkForInnerConflicts() in parallel and
// does some printing.
func checkForAllConflicts(dc *depCatagories) error {
	var err error
	var conflicts map[string]stringSet
	var innerConflicts map[string]stringSet
	var wg sync.WaitGroup
	wg.Add(2)

	fmt.Println(bold(cyan("::") + bold(" Checking for conflicts...")))
	go func() {
		conflicts, err = checkForConflicts(dc)
		wg.Done()
	}()

	fmt.Println(bold(cyan("::") + bold(" Checking for inner conflicts...")))
	go func() {
		innerConflicts = checkForInnerConflicts(dc)
		wg.Done()
	}()

	wg.Wait()

	if err != nil {
		return err
	}

	if len(innerConflicts) != 0 {
		fmt.Println()
		fmt.Println(bold(red(arrow)), bold("Inner conflicts found:"))

		for name, pkgs := range innerConflicts {
			str := red(bold(smallArrow)) + " " + name + ":"
			for pkg := range pkgs {
				str += " " + cyan(pkg)
			}

			fmt.Println(str)
		}

		return fmt.Errorf("Unresolvable package conflicts, aborting")
	}

	if len(conflicts) != 0 {
		fmt.Println()
		fmt.Println(bold(red(arrow)), bold("Package conflicts found:"))
		for name, pkgs := range conflicts {
			str := red(bold(smallArrow)) + " Installing " + cyan(name) + " will remove:"
			for pkg := range pkgs {
				str += " " + cyan(pkg)
			}

			fmt.Println(str)
		}

		fmt.Println()
	}

	return nil
}

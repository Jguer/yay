package main

import (
	"fmt"
	"strings"

	rpc "github.com/mikkeloscar/aur"
)

// BuildDependencies finds packages, on the second run
// compares with a baselist and avoids searching those
func buildDependencies(baselist []string) func(toCheck []string, isBaseList bool, last bool) (repo []string, notFound []string) {
	localDb, err := AlpmHandle.LocalDb()
	if err != nil {
		panic(err)
	}

	dbList, err := AlpmHandle.SyncDbs()
	if err != nil {
		panic(err)
	}

	f := func(c rune) bool {
		return c == '>' || c == '<' || c == '=' || c == ' '
	}

	return func(toCheck []string, isBaseList bool, close bool) (repo []string, notFound []string) {
		if close {
			return
		}

	Loop:
		for _, dep := range toCheck {
			if !isBaseList {
				for _, base := range baselist {
					if base == dep {
						continue Loop
					}
				}
			}
			if _, erp := localDb.PkgCache().FindSatisfier(dep); erp == nil {
				continue
			} else if pkg, erp := dbList.FindSatisfier(dep); erp == nil {
				repo = append(repo, pkg.Name())
			} else {
				field := strings.FieldsFunc(dep, f)
				notFound = append(notFound, field[0])
			}
		}
		return
	}
}

// DepSatisfier receives a string slice, returns a slice of packages found in
// repos and one of packages not found in repos. Leaves out installed packages.
func depSatisfier(toCheck []string) (repo []string, notFound []string, err error) {
	localDb, err := AlpmHandle.LocalDb()
	if err != nil {
		return
	}
	dbList, err := AlpmHandle.SyncDbs()
	if err != nil {
		return
	}

	f := func(c rune) bool {
		return c == '>' || c == '<' || c == '=' || c == ' '
	}

	for _, dep := range toCheck {
		if _, erp := localDb.PkgCache().FindSatisfier(dep); erp == nil {
			continue
		} else if pkg, erp := dbList.FindSatisfier(dep); erp == nil {
			repo = append(repo, pkg.Name())
		} else {
			field := strings.FieldsFunc(dep, f)
			notFound = append(notFound, field[0])
		}
	}

	err = nil
	return
}

// PkgDependencies returns package dependencies not installed belonging to AUR
// 0 is Repo, 1 is Foreign.
func pkgDependencies(a *rpc.Pkg) (runDeps [2][]string, makeDeps [2][]string, err error) {
	var q aurQuery
	if len(a.Depends) == 0 && len(a.MakeDepends) == 0 {
		q, err = rpc.Info([]string{a.Name})
		if len(q) == 0 || err != nil {
			err = fmt.Errorf("Unable to search dependencies, %s", err)
			return
		}
	} else {
		q = append(q, *a)
	}

	depSearch := buildDependencies(a.Depends)
	if len(a.Depends) != 0 {
		runDeps[0], runDeps[1] = depSearch(q[0].Depends, true, false)
		if len(runDeps[0]) != 0 || len(runDeps[1]) != 0 {
			fmt.Println("\x1b[1;32m=>\x1b[1;33m Run Dependencies: \x1b[0m")
			printDeps(runDeps[0], runDeps[1])
		}
	}

	if len(a.MakeDepends) != 0 {
		makeDeps[0], makeDeps[1] = depSearch(q[0].MakeDepends, false, false)
		if len(makeDeps[0]) != 0 || len(makeDeps[1]) != 0 {
			fmt.Println("\x1b[1;32m=>\x1b[1;33m Make Dependencies: \x1b[0m")
			printDeps(makeDeps[0], makeDeps[1])
		}
	}
	depSearch(a.MakeDepends, false, true)

	err = nil
	return
}

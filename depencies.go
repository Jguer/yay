package main

import "strings"

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

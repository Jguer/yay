package main

import (
	"fmt"
)

const gitEmptyTree = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

func stringSliceEqual(a, b []string) bool {
	if a == nil && b == nil {
		return true
	}

	if a == nil || b == nil {
		return false
	}

	if len(a) != len(b) {
		return false
	}

	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

func removeInvalidTargets(targets []string) []string {
	filteredTargets := make([]string, 0)

	for _, target := range targets {
		db, _ := splitDBFromName(target)

		if db == "aur" && mode == modeRepo {
			fmt.Printf("%s %s %s\n", bold(yellow(arrow)), cyan(target), bold("Can't use target with option --repo -- skipping"))
			continue
		}

		if db != "aur" && db != "" && mode == modeAUR {
			fmt.Printf("%s %s %s\n", bold(yellow(arrow)), cyan(target), bold("Can't use target with option --aur -- skipping"))
			continue
		}

		filteredTargets = append(filteredTargets, target)
	}

	return filteredTargets
}

package main

import (
	"fmt"
	"syscall"
	"unicode"
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

// LessRunes compares two rune values, and returns true if the first argument is lexicographicaly smaller.
func LessRunes(iRunes, jRunes []rune) bool {
	max := len(iRunes)
	if max > len(jRunes) {
		max = len(jRunes)
	}

	for idx := 0; idx < max; idx++ {
		ir := iRunes[idx]
		jr := jRunes[idx]

		lir := unicode.ToLower(ir)
		ljr := unicode.ToLower(jr)

		if lir != ljr {
			return lir < ljr
		}

		// the lowercase runes are the same, so compare the original
		if ir != jr {
			return ir < jr
		}
	}

	return len(iRunes) < len(jRunes)
}

const ioprioClassIdle = 3
const ioprioWhoProcess = 1
const ioprioClassShift = 13

func ioprioPrioValue(class, data int) int { return ((class) << ioprioClassShift) | data }

func ioPrioSet(which, who, ioprio int) int {
	ecode, _, _ := syscall.Syscall(syscall.SYS_IOPRIO_SET, uintptr(which), uintptr(who), uintptr(ioprio))
	return int(ecode)
}

const prioProcess = 0

func setPriority(which, who, prio int) int {
	ecode, _, _ := syscall.Syscall(syscall.SYS_SETPRIORITY, uintptr(which), uintptr(who), uintptr(prio))
	return int(ecode)
}

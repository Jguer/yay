package main

import (
	"fmt"
	"sync"
	"unicode"
)

const gitEmptyTree = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

type mapStringSet map[string]stringSet

type intRange struct {
	min int
	max int
}

func makeIntRange(min, max int) intRange {
	return intRange{
		min,
		max,
	}
}

func (r intRange) get(n int) bool {
	return n >= r.min && n <= r.max
}

type intRanges []intRange

func (rs intRanges) get(n int) bool {
	for _, r := range rs {
		if r.get(n) {
			return true
		}
	}

	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a < b {
		return b
	}
	return a
}

func (mss mapStringSet) Add(n string, v string) {
	_, ok := mss[n]
	if !ok {
		mss[n] = make(stringSet)
	}
	mss[n].set(v)
}

func lessRunes(iRunes, jRunes []rune) bool {
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

type MultiError struct {
	Errors []error
	mux    sync.Mutex
}

func (err *MultiError) Error() string {
	str := ""

	for _, e := range err.Errors {
		str += e.Error() + "\n"
	}

	return str[:len(str)-1]
}

func (err *MultiError) Add(e error) {
	if e == nil {
		return
	}

	err.mux.Lock()
	err.Errors = append(err.Errors, e)
	err.mux.Unlock()
}

func (err *MultiError) Return() error {
	if len(err.Errors) > 0 {
		return err
	}

	return nil
}

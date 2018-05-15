package pkgbuild

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// Version string
type Version string

type CompleteVersion struct {
	Version Version
	Epoch   uint8
	Pkgrel  Version
}

func (c *CompleteVersion) String() string {
	str := ""

	if c.Epoch > 0 {
		str = fmt.Sprintf("%d:", c.Epoch)
	}

	str = fmt.Sprintf("%s%s", str, c.Version)

	if c.Pkgrel != "" {
		str = fmt.Sprintf("%s-%s", str, c.Pkgrel)
	}

	return str
}

// NewCompleteVersion creates a CompleteVersion including basic version, epoch
// and rel from string
func NewCompleteVersion(s string) (*CompleteVersion, error) {
	var err error
	epoch := 0
	rel := Version("")

	// handle possible epoch
	versions := strings.Split(s, ":")
	if len(versions) > 2 {
		return nil, fmt.Errorf("invalid version format: %s", s)
	}

	if len(versions) > 1 {
		epoch, err = strconv.Atoi(versions[0])
		if err != nil {
			return nil, err
		}
	}

	// handle possible rel
	versions = strings.Split(versions[len(versions)-1], "-")
	if len(versions) > 2 {
		return nil, fmt.Errorf("invalid version format: %s", s)
	}

	if len(versions) > 1 {
		rel = Version(versions[1])
	}

	// finally check that the actual version is valid
	if validPkgver(versions[0]) {
		return &CompleteVersion{
			Version: Version(versions[0]),
			Epoch:   uint8(epoch),
			Pkgrel:  rel,
		}, nil
	}

	return nil, fmt.Errorf("invalid version format: %s", s)
}

// Older returns true if a is older than the argument version
func (a *CompleteVersion) Older(b *CompleteVersion) bool {
	return a.cmp(b) == -1
}

// Newer returns true if a is newer than the argument version
func (a *CompleteVersion) Newer(b *CompleteVersion) bool {
	return a.cmp(b) == 1
}

// Equal returns true if a is equal to the argument version
func (a *CompleteVersion) Equal(b *CompleteVersion) bool {
	return a.cmp(b) == 0
}

// Satisfies tests whether or not version fits inside the bounds specified by
// dep
func (version *CompleteVersion) Satisfies(dep *Dependency) bool {
	var cmpMax int8
	var cmpMin int8

	if dep.MaxVer != nil {
		cmpMax = version.cmp(dep.MaxVer)
		if cmpMax == 1 {
			return false
		}

		if cmpMax == 0 && dep.slt {
			return false
		}
	}

	if dep.MinVer != nil {
		if dep.MaxVer == dep.MinVer {
			cmpMin = cmpMax
		} else {
			cmpMin = version.cmp(dep.MinVer)
		}
		if cmpMin == -1 {
			return false
		}

		if cmpMin == 0 && dep.sgt {
			return false
		}
	}

	return true
}

// Compare a to b:
// return 1: a is newer than b
//        0: a and b are the same version
//       -1: b is newer than a
func (a *CompleteVersion) cmp(b *CompleteVersion) int8 {
	if a.Epoch > b.Epoch {
		return 1
	}

	if a.Epoch < b.Epoch {
		return -1
	}

	if a.Version.bigger(b.Version) {
		return 1
	}

	if b.Version.bigger(a.Version) {
		return -1
	}

	if a.Pkgrel == "" || b.Pkgrel == "" {
		return 0
	}

	if a.Pkgrel.bigger(b.Pkgrel) {
		return 1
	}

	if b.Pkgrel.bigger(a.Pkgrel) {
		return -1
	}

	return 0
}

// Compare alpha and numeric segments of two versions.
// return 1: a is newer than b
//        0: a and b are the same version
//       -1: b is newer than a
//
// This is based on the rpmvercmp function used in libalpm
// https://projects.archlinux.org/pacman.git/tree/lib/libalpm/version.c
func rpmvercmp(av, bv Version) int {
	if av == bv {
		return 0
	}
	a, b := []rune(string(av)), []rune(string(bv))

	var one, two, ptr1, ptr2 int
	var isNum bool
	one, two, ptr1, ptr2 = 0, 0, 0, 0

	// loop through each version segment of a and b and compare them
	for len(a) > one && len(b) > two {
		for len(a) > one && !isAlphaNumeric(a[one]) {
			one++
		}
		for len(b) > two && !isAlphaNumeric(b[two]) {
			two++
		}

		// if we ran to the end of either, we are finished with the loop
		if !(len(a) > one && len(b) > two) {
			break
		}

		// if the seperator lengths were different, we are also finished
		if one-ptr1 != two-ptr2 {
			if one-ptr1 < two-ptr2 {
				return -1
			}
			return 1
		}

		ptr1 = one
		ptr2 = two

		// grab first completely alpha or completely numeric segment
		// leave one and two pointing to the start of the alpha or numeric
		// segment and walk ptr1 and ptr2 to end of segment
		if isDigit(a[ptr1]) {
			for len(a) > ptr1 && isDigit(a[ptr1]) {
				ptr1++
			}
			for len(b) > ptr2 && isDigit(b[ptr2]) {
				ptr2++
			}
			isNum = true
		} else {
			for len(a) > ptr1 && isAlpha(a[ptr1]) {
				ptr1++
			}
			for len(b) > ptr2 && isAlpha(b[ptr2]) {
				ptr2++
			}
			isNum = false
		}

		// take care of the case where the two version segments are
		// different types: one numeric, the other alpha (i.e. empty)
		// numeric segments are always newer than alpha segments
		if two == ptr2 {
			if isNum {
				return 1
			}
			return -1
		}

		if isNum {
			// we know this part of the strings only contains digits
			// so we can ignore the error value since it should
			// always be nil
			as, _ := strconv.ParseInt(string(a[one:ptr1]), 10, 0)
			bs, _ := strconv.ParseInt(string(b[two:ptr2]), 10, 0)

			// whichever number has more digits wins
			if as > bs {
				return 1
			}
			if as < bs {
				return -1
			}
		} else {
			cmp := alphaCompare(a[one:ptr1], b[two:ptr2])
			if cmp < 0 {
				return -1
			}
			if cmp > 0 {
				return 1
			}
		}

		// advance one and two to next segment
		one = ptr1
		two = ptr2
	}

	// this catches the case where all numeric and alpha segments have
	// compared identically but the segment separating characters were
	// different
	if len(a) <= one && len(b) <= two {
		return 0
	}

	// the final showdown. we never want a remaining alpha string to
	// beat an empty string. the logic is a bit weird, but:
	// - if one is empty and two is not an alpha, two is newer.
	// - if one is an alpha, two is newer.
	// - otherwise one is newer.
	if (len(a) <= one && !isAlpha(b[two])) || len(a) > one && isAlpha(a[one]) {
		return -1
	}
	return 1
}

// alphaCompare compares two alpha version segments and will return a positive
// value if a is bigger than b and a negative if b is bigger than a else 0
func alphaCompare(a, b []rune) int8 {
	if string(a) == string(b) {
		return 0
	}

	i := 0
	for len(a) > i && len(b) > i && a[i] == b[i] {
		i++
	}

	if len(a) == i && len(b) > i {
		return -1
	}

	if len(b) == i {
		return 1
	}

	return int8(a[i]) - int8(b[i])
}

// check if version number v is bigger than v2
func (v Version) bigger(v2 Version) bool {
	return rpmvercmp(v, v2) == 1
}

// isAlphaNumeric reports whether c is an alpha character or digit
func isAlphaNumeric(c rune) bool {
	return isDigit(c) || isAlpha(c)
}

// isAlpha reports whether c is an alpha character
func isAlpha(c rune) bool {
	return unicode.IsLetter(c)
}

// isDigit reports whether d is an ASCII digit
func isDigit(d rune) bool {
	return unicode.IsDigit(d)
}

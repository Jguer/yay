package intrange

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/Jguer/yay/v10/pkg/stringset"
)

// IntRange stores a max and min amount for range
type IntRange struct {
	min int
	max int
}

// IntRanges is a slice of IntRange
type IntRanges []IntRange

func makeIntRange(min, max int) IntRange {
	return IntRange{
		min,
		max,
	}
}

// Get returns true if the argument n is included in the closed range
// between min and max
func (r IntRange) Get(n int) bool {
	return n >= r.min && n <= r.max
}

// Get returns true if the argument n is included in the closed range
// between min and max of any of the provided IntRanges
func (rs IntRanges) Get(n int) bool {
	for _, r := range rs {
		if r.Get(n) {
			return true
		}
	}

	return false
}

// Min returns min value between a and b
func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Max returns max value between a and b
func Max(a, b int) int {
	if a < b {
		return b
	}
	return a
}

// ParseNumberMenu parses input for number menus split by spaces or commas
// supports individual selection: 1 2 3 4
// supports range selections: 1-4 10-20
// supports negation: ^1 ^1-4
//
// include and excule holds numbers that should be added and should not be added
// respectively. other holds anything that can't be parsed as an int. This is
// intended to allow words inside of number menus. e.g. 'all' 'none' 'abort'
// of course the implementation is up to the caller, this function mearley parses
// the input and organizes it
func ParseNumberMenu(input string) (include, exclude IntRanges,
	otherInclude, otherExclude stringset.StringSet) {
	include = make(IntRanges, 0)
	exclude = make(IntRanges, 0)
	otherInclude = make(stringset.StringSet)
	otherExclude = make(stringset.StringSet)

	words := strings.FieldsFunc(input, func(c rune) bool {
		return unicode.IsSpace(c) || c == ','
	})

	for _, word := range words {
		var num1 int
		var num2 int
		var err error
		invert := false
		other := otherInclude

		if word[0] == '^' {
			invert = true
			other = otherExclude
			word = word[1:]
		}

		ranges := strings.SplitN(word, "-", 2)

		num1, err = strconv.Atoi(ranges[0])
		if err != nil {
			other.Set(strings.ToLower(word))
			continue
		}

		if len(ranges) == 2 {
			num2, err = strconv.Atoi(ranges[1])
			if err != nil {
				other.Set(strings.ToLower(word))
				continue
			}
		} else {
			num2 = num1
		}

		mi := Min(num1, num2)
		ma := Max(num1, num2)

		if !invert {
			include = append(include, makeIntRange(mi, ma))
		} else {
			exclude = append(exclude, makeIntRange(mi, ma))
		}
	}

	return include, exclude, otherInclude, otherExclude
}

package main

import (
	"testing"

	"github.com/Jguer/yay/v9/pkg/types"
)

func intRangesEqual(a, b intRanges) bool {
	if a == nil && b == nil {
		return true
	}

	if a == nil || b == nil {
		return false
	}

	if len(a) != len(b) {
		return false
	}

	for n := range a {
		r1 := a[n]
		r2 := b[n]

		if r1.min != r2.min || r1.max != r2.max {
			return false
		}
	}

	return true
}

func TestParseNumberMenu(t *testing.T) {
	type result struct {
		Include      intRanges
		Exclude      intRanges
		OtherInclude types.StringSet
		OtherExclude types.StringSet
	}

	inputs := []string{
		"1 2 3 4 5",
		"1-10 5-15",
		"10-5 90-85",
		"1 ^2 ^10-5 99 ^40-38 ^123 60-62",
		"abort all none",
		"a-b ^a-b ^abort",
		"1\t2   3      4\t\t  \t 5",
		"1 2,3, 4,  5,6 ,7  ,8",
		"",
		"   \t   ",
		"A B C D E",
	}

	expected := []result{
		{intRanges{makeIntRange(1, 1), makeIntRange(2, 2), makeIntRange(3, 3), makeIntRange(4, 4), makeIntRange(5, 5)}, intRanges{}, make(types.StringSet), make(types.StringSet)},
		{intRanges{makeIntRange(1, 10), makeIntRange(5, 15)}, intRanges{}, make(types.StringSet), make(types.StringSet)},
		{intRanges{makeIntRange(5, 10), makeIntRange(85, 90)}, intRanges{}, make(types.StringSet), make(types.StringSet)},
		{intRanges{makeIntRange(1, 1), makeIntRange(99, 99), makeIntRange(60, 62)}, intRanges{makeIntRange(2, 2), makeIntRange(5, 10), makeIntRange(38, 40), makeIntRange(123, 123)}, make(types.StringSet), make(types.StringSet)},
		{intRanges{}, intRanges{}, types.MakeStringSet("abort", "all", "none"), make(types.StringSet)},
		{intRanges{}, intRanges{}, types.MakeStringSet("a-b"), types.MakeStringSet("abort", "a-b")},
		{intRanges{makeIntRange(1, 1), makeIntRange(2, 2), makeIntRange(3, 3), makeIntRange(4, 4), makeIntRange(5, 5)}, intRanges{}, make(types.StringSet), make(types.StringSet)},
		{intRanges{makeIntRange(1, 1), makeIntRange(2, 2), makeIntRange(3, 3), makeIntRange(4, 4), makeIntRange(5, 5), makeIntRange(6, 6), makeIntRange(7, 7), makeIntRange(8, 8)}, intRanges{}, make(types.StringSet), make(types.StringSet)},
		{intRanges{}, intRanges{}, make(types.StringSet), make(types.StringSet)},
		{intRanges{}, intRanges{}, make(types.StringSet), make(types.StringSet)},
		{intRanges{}, intRanges{}, types.MakeStringSet("a", "b", "c", "d", "e"), make(types.StringSet)},
	}

	for n, in := range inputs {
		res := expected[n]
		include, exclude, otherInclude, otherExclude := parseNumberMenu(in)

		if !intRangesEqual(include, res.Include) ||
			!intRangesEqual(exclude, res.Exclude) ||
			!types.StringSetEqual(otherInclude, res.OtherInclude) ||
			!types.StringSetEqual(otherExclude, res.OtherExclude) {

			t.Fatalf("Test %d Failed: Expected: include=%+v exclude=%+v otherInclude=%+v otherExclude=%+v got include=%+v excluive=%+v otherInclude=%+v otherExclude=%+v",
				n+1, res.Include, res.Exclude, res.OtherInclude, res.OtherExclude, include, exclude, otherInclude, otherExclude)
		}
	}
}

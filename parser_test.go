package main

import "testing"

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

func stringSetEqual(a, b stringSet) bool {
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
		if !b.get(n) {
			return false
		}
	}

	return true
}

func TestParseNumberMenu(t *testing.T) {
	type result struct {
		Include      intRanges
		Exclude      intRanges
		OtherInclude stringSet
		OtherExclude stringSet
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
		{intRanges{makeIntRange(1, 1), makeIntRange(2, 2), makeIntRange(3, 3), makeIntRange(4, 4), makeIntRange(5, 5)}, intRanges{}, make(stringSet), make(stringSet)},
		{intRanges{makeIntRange(1, 10), makeIntRange(5, 15)}, intRanges{}, make(stringSet), make(stringSet)},
		{intRanges{makeIntRange(5, 10), makeIntRange(85, 90)}, intRanges{}, make(stringSet), make(stringSet)},
		{intRanges{makeIntRange(1, 1), makeIntRange(99, 99), makeIntRange(60, 62)}, intRanges{makeIntRange(2, 2), makeIntRange(5, 10), makeIntRange(38, 40), makeIntRange(123, 123)}, make(stringSet), make(stringSet)},
		{intRanges{}, intRanges{}, makeStringSet("abort", "all", "none"), make(stringSet)},
		{intRanges{}, intRanges{}, makeStringSet("a-b"), makeStringSet("abort", "a-b")},
		{intRanges{makeIntRange(1, 1), makeIntRange(2, 2), makeIntRange(3, 3), makeIntRange(4, 4), makeIntRange(5, 5)}, intRanges{}, make(stringSet), make(stringSet)},
		{intRanges{makeIntRange(1, 1), makeIntRange(2, 2), makeIntRange(3, 3), makeIntRange(4, 4), makeIntRange(5, 5), makeIntRange(6, 6), makeIntRange(7, 7), makeIntRange(8, 8)}, intRanges{}, make(stringSet), make(stringSet)},
		{intRanges{}, intRanges{}, make(stringSet), make(stringSet)},
		{intRanges{}, intRanges{}, make(stringSet), make(stringSet)},
		{intRanges{}, intRanges{}, makeStringSet("a", "b", "c", "d", "e"), make(stringSet)},
	}

	for n, in := range inputs {
		res := expected[n]
		include, exclude, otherInclude, otherExclude := parseNumberMenu(in)

		if !intRangesEqual(include, res.Include) ||
			!intRangesEqual(exclude, res.Exclude) ||
			!stringSetEqual(otherInclude, res.OtherInclude) ||
			!stringSetEqual(otherExclude, res.OtherExclude) {

			t.Fatalf("Test %d Failed: Expected: include=%+v exclude=%+v otherInclude=%+v otherExclude=%+v got include=%+v excluive=%+v otherInclude=%+v otherExclude=%+v",
				n+1, res.Include, res.Exclude, res.OtherInclude, res.OtherExclude, include, exclude, otherInclude, otherExclude)
		}
	}
}

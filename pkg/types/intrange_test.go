package types

import "testing"

func intRangesEqual(a, b IntRanges) bool {
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

func stringSetEqual(a, b StringSet) bool {
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
		if !b.Get(n) {
			return false
		}
	}

	return true
}

func TestParseNumberMenu(t *testing.T) {
	type result struct {
		Include      IntRanges
		Exclude      IntRanges
		OtherInclude StringSet
		OtherExclude StringSet
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
		{IntRanges{makeIntRange(1, 1), makeIntRange(2, 2), makeIntRange(3, 3), makeIntRange(4, 4), makeIntRange(5, 5)}, IntRanges{}, make(StringSet), make(StringSet)},
		{IntRanges{makeIntRange(1, 10), makeIntRange(5, 15)}, IntRanges{}, make(StringSet), make(StringSet)},
		{IntRanges{makeIntRange(5, 10), makeIntRange(85, 90)}, IntRanges{}, make(StringSet), make(StringSet)},
		{IntRanges{makeIntRange(1, 1), makeIntRange(99, 99), makeIntRange(60, 62)}, IntRanges{makeIntRange(2, 2), makeIntRange(5, 10), makeIntRange(38, 40), makeIntRange(123, 123)}, make(StringSet), make(StringSet)},
		{IntRanges{}, IntRanges{}, MakeStringSet("abort", "all", "none"), make(StringSet)},
		{IntRanges{}, IntRanges{}, MakeStringSet("a-b"), MakeStringSet("abort", "a-b")},
		{IntRanges{makeIntRange(1, 1), makeIntRange(2, 2), makeIntRange(3, 3), makeIntRange(4, 4), makeIntRange(5, 5)}, IntRanges{}, make(StringSet), make(StringSet)},
		{IntRanges{makeIntRange(1, 1), makeIntRange(2, 2), makeIntRange(3, 3), makeIntRange(4, 4), makeIntRange(5, 5), makeIntRange(6, 6), makeIntRange(7, 7), makeIntRange(8, 8)}, IntRanges{}, make(StringSet), make(StringSet)},
		{IntRanges{}, IntRanges{}, make(StringSet), make(StringSet)},
		{IntRanges{}, IntRanges{}, make(StringSet), make(StringSet)},
		{IntRanges{}, IntRanges{}, MakeStringSet("a", "b", "c", "d", "e"), make(StringSet)},
	}

	for n, in := range inputs {
		res := expected[n]
		include, exclude, otherInclude, otherExclude := ParseNumberMenu(in)

		if !intRangesEqual(include, res.Include) ||
			!intRangesEqual(exclude, res.Exclude) ||
			!stringSetEqual(otherInclude, res.OtherInclude) ||
			!stringSetEqual(otherExclude, res.OtherExclude) {

			t.Fatalf("Test %d Failed: Expected: include=%+v exclude=%+v otherInclude=%+v otherExclude=%+v got include=%+v excluive=%+v otherInclude=%+v otherExclude=%+v",
				n+1, res.Include, res.Exclude, res.OtherInclude, res.OtherExclude, include, exclude, otherInclude, otherExclude)
		}
	}
}

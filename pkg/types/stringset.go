package types

// StringSet is a basic set implementation for strings.
// This is used a lot so it deserves its own type.
// Other types of sets are used throughout the code but do not have
// their own typedef.
// String sets and <type>sets should be used throughout the code when applicable,
// they are a lot more flexible than slices and provide easy lookup.
type StringSet map[string]struct{}

type MapStringSet map[string]StringSet

func (mss MapStringSet) Add(n string, v string) {
	_, ok := mss[n]
	if !ok {
		mss[n] = make(StringSet)
	}
	mss[n].Set(v)
}

func (set StringSet) Set(v string) {
	set[v] = struct{}{}
}

func (set StringSet) Get(v string) bool {
	_, exists := set[v]
	return exists
}

func (set StringSet) Remove(v string) {
	delete(set, v)
}

func (set StringSet) ToSlice() []string {
	slice := make([]string, 0, len(set))

	for v := range set {
		slice = append(slice, v)
	}

	return slice
}

func (set StringSet) Copy() StringSet {
	newSet := make(StringSet)

	for str := range set {
		newSet.Set(str)
	}

	return newSet
}

func SliceToStringSet(in []string) StringSet {
	set := make(StringSet)

	for _, v := range in {
		set.Set(v)
	}

	return set
}

func MakeStringSet(in ...string) StringSet {
	return SliceToStringSet(in)
}

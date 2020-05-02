package stringset

// StringSet is a basic set implementation for strings.
// This is used a lot so it deserves its own type.
// Other types of sets are used throughout the code but do not have
// their own typedef.
// String sets and <type>sets should be used throughout the code when applicable,
// they are a lot more flexible than slices and provide easy lookup.
type StringSet map[string]struct{}

// MapStringSet is a Map of StringSets.
type MapStringSet map[string]StringSet

// Add adds a new value to the Map.
// If n is already in the map, then v is appended to the StringSet under that key.
// Otherwise a new StringSet is creayed containing v
func (mss MapStringSet) Add(n, v string) {
	_, ok := mss[n]
	if !ok {
		mss[n] = make(StringSet)
	}
	mss[n].Set(v)
}

// Set sets key in StringSet.
func (set StringSet) Set(v string) {
	set[v] = struct{}{}
}

// Extend sets multiple keys in StringSet.
func (set StringSet) Extend(s ...string) {
	for _, v := range s {
		set[v] = struct{}{}
	}
}

// Get returns true if the key exists in the set.
func (set StringSet) Get(v string) bool {
	_, exists := set[v]
	return exists
}

// Remove deletes a key from the set.
func (set StringSet) Remove(v string) {
	delete(set, v)
}

// ToSlice turns all keys into a string slice.
func (set StringSet) ToSlice() []string {
	slice := make([]string, 0, len(set))

	for v := range set {
		slice = append(slice, v)
	}

	return slice
}

// Copy copies a StringSet into a new structure of the same type.
func (set StringSet) Copy() StringSet {
	newSet := make(StringSet)

	for str := range set {
		newSet.Set(str)
	}

	return newSet
}

// FromSlice creates a new StringSet from an input slice
func FromSlice(in []string) StringSet {
	set := make(StringSet)

	for _, v := range in {
		set.Set(v)
	}

	return set
}

// Make creates a new StringSet from a set of arguments
func Make(in ...string) StringSet {
	return FromSlice(in)
}

// Equal compares if two StringSets have the same values
func Equal(a, b StringSet) bool {
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

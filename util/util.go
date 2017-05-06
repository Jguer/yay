package util

// SearchVerbosity determines print method used in PrintSearch
var SearchVerbosity = NumberMenu

// Verbosity settings for search
const (
	NumberMenu = iota
	Detailed
	Minimal
)

// Build controls if packages will be built from ABS.
var Build = false

// NoConfirm ignores prompts.
var NoConfirm = false

// SortMode determines top down package or down top package display
var SortMode = BottomUp

// Describes Sorting method for numberdisplay
const (
	BottomUp = iota
	TopDown
)

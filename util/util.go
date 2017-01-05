package util

import "fmt"

// TarBin describes the default installation point of tar command.
const TarBin string = "/usr/bin/tar"

// MakepkgBin describes the default installation point of makepkg command.
const MakepkgBin string = "/usr/bin/makepkg"

// SearchVerbosity determines print method used in PrintSearch
var SearchVerbosity = NumberMenu

// Verbosity settings for search
const (
	NumberMenu = iota
	Detailed
	Minimal
)

// NoConfirm ignores prompts.
var NoConfirm = false

// SortMode determines top down package or down top package display
var SortMode = BottomUp

// BaseDir is the default building directory for yay
var BaseDir = "/tmp/yaytmp/"

// Describes Sorting method for numberdisplay
const (
	BottomUp = iota
	TopDown
)

// ContinueTask prompts if user wants to continue task.
//If NoConfirm is set the action will continue without user input.
func ContinueTask(s string, def string) (cont bool) {
	if NoConfirm {
		return true
	}
	var postFix string

	if def == "nN" {
		postFix = "(Y/n)"
	} else {
		postFix = "(y/N)"
	}

	var response string
	fmt.Printf("\x1b[1;32m==> %s\x1b[1;37m %s\x1b[0m\n", s, postFix)

	fmt.Scanln(&response)
	if response == string(def[0]) || response == string(def[1]) {
		return false
	}

	return true
}

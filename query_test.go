package main

import (
	"testing"
)

// -----------------------------
// validPkgName
// -----------------------------

func TestValidPkgName_WhenTestedPackageNameContainsAllSearched_ReturnsTrue(t *testing.T) {

	searchedPkgNames := []string{"one", "two", "three"}
	pkgName := "zeroOneTwoThreeFour"

	if !validPkgName(searchedPkgNames, pkgName) {
		t.Fatalf("Tested package name (%s) scontains all searched package names (%s)", searchedPkgNames, pkgName)
	}
}

func TestValidPkgName_WhenTestedPackageNameDoesNotContainAllSearched_ReturnsFalse(t *testing.T) {

	searchedPkgNames := []string{"one", "two", "three"}
	pkgName := "zeroOneThreeFour"

	if validPkgName(searchedPkgNames, pkgName) {
		t.Fatalf("Tested package name (%s) does not contain searched package name 'two'", searchedPkgNames)
	}
}

func TestValidPkgName_WhenTestedPackageNameIsEmpty_ReturnsFalse(t *testing.T) {

	searchedPkgNames := []string{"one", "two", "three"}
	pkgName := ""

	if validPkgName(searchedPkgNames, pkgName) {
		t.Fatalf("Tested package name is empty")
	}
}

func TestValidPkgName_WhenSearchedPackageNamesIsEmpty_ReturnsTrue(t *testing.T) {

	var searchedPkgNames []string
	pkgName := "test"

	if !validPkgName(searchedPkgNames, pkgName) {
		t.Fatalf("Searched package names is empty, all results are valid.")
	}
}

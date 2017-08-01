// alpm_test.go - Tests for alpm.go.
//
// Copyright (c) 2013 The go-alpm Authors
//
// MIT Licensed. See LICENSE for details.

package alpm

import (
	"fmt"
	"os"
	"testing"
)

const (
	root   = "/"
	dbpath = "/var/lib/pacman"
)

var h *Handle

func init() {
	var err error
	h, err = Init("/", "/var/lib/pacman")
	if err != nil {
		fmt.Printf("failed to Init(): %s", err)
		os.Exit(1)
	}
}

func ExampleVersion() {
	fmt.Println(Version())
	// output:
	// 8.0.2
}

func ExampleVerCmp() {
	fmt.Println(VerCmp("1.0-2", "2.0-1") < 0)
	fmt.Println(VerCmp("1:1.0-2", "2.0-1") > 0)
	fmt.Println(VerCmp("2.0.2-2", "2.0.2-2") == 0)
	// output:
	// true
	// true
	// true
}

func TestRevdeps(t *testing.T) {
	db, _ := h.LocalDb()
	pkg, _ := db.PkgByName("glibc")
	for i, pkgname := range pkg.ComputeRequiredBy() {
		t.Logf(pkgname)
		if i == 10 {
			t.Logf("and %d more...", len(pkg.ComputeRequiredBy())-10)
			return
		}
	}
}

func TestLocalDB(t *testing.T) {
	defer func() {
		if recover() != nil {
			t.Errorf("local db failed")
		}
	}()
	db, _ := h.LocalDb()
	number := 0
	for _, pkg := range db.PkgCache().Slice() {
		number++
		if number <= 15 {
			t.Logf("%v", pkg.Name())
		}
	}
	if number > 15 {
		t.Logf("%d more packages...", number-15)
	}
}

func TestRelease(t *testing.T) {
	if err := h.Release(); err != nil {
		t.Error(err)
	}
}

package main

import "github.com/leonelquinteros/gotext"

type UnableToFindPkgDestError struct {
	name, pkgDest string
}

func (e *UnableToFindPkgDestError) Error() string {
	return gotext.Get(
		"the PKGDEST for %s is listed by makepkg but does not exist: %s",
		e.name, e.pkgDest)
}

type PkgDestNotInListError struct {
	name string
}

func (e *PkgDestNotInListError) Error() string {
	return gotext.Get("could not find PKGDEST for: %s", e.name)
}

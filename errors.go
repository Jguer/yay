package main

import "github.com/leonelquinteros/gotext"

type SetPkgReasonError struct {
	exp bool // explicit
}

func (e *SetPkgReasonError) Error() string {
	reason := gotext.Get("explicit")
	if !e.exp {
		reason = gotext.Get("dependency")
	}

	return gotext.Get("error updating package install reason to %s", reason)
}

type FindPkgDestError struct {
	name, pkgDest string
}

func (e *FindPkgDestError) Error() string {
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

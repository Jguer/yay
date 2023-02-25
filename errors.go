package main

import (
	"errors"

	"github.com/leonelquinteros/gotext"
)

var ErrPackagesNotFound = errors.New(gotext.Get("could not find all required packages"))

type NoPkgDestsFoundError struct {
	dir string
}

func (e *NoPkgDestsFoundError) Error() string {
	return gotext.Get("could not find any package archives listed in %s", e.dir)
}

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

type FailedIgnoredPkgError struct {
	pkgErrors map[string]error
}

func (e *FailedIgnoredPkgError) Error() string {
	msg := gotext.Get("Failed to install the following packages. Manual intervention is required:")

	for pkg, err := range e.pkgErrors {
		msg += "\n" + pkg + " - " + err.Error()
	}

	return msg
}

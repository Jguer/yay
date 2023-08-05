package main

import (
	"errors"

	"github.com/leonelquinteros/gotext"
)

var ErrPackagesNotFound = errors.New(gotext.Get("could not find all required packages"))

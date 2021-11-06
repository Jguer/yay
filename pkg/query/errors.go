package query

import (
	"github.com/leonelquinteros/gotext"
)

// ErrAURSearch means that it was not possible to connect to the AUR.
type ErrAURSearch struct {
	inner error
}

func (e ErrAURSearch) Error() string {
	return gotext.Get("Error during AUR search: %s\n", e.inner.Error())
}

// ErrNoQuery means that query was not executed.
type ErrNoQuery struct{}

func (e ErrNoQuery) Error() string {
	return gotext.Get("no query was executed")
}

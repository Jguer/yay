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

// ErrInvalidSortMode means that the sort mode provided was not valid.
type ErrInvalidSortMode struct{}

func (e ErrInvalidSortMode) Error() string {
	return gotext.Get("invalid sort mode. Fix with yay -Y --bottomup --save")
}

// ErrNoQuery means that query was not executed.
type ErrNoQuery struct{}

func (e ErrNoQuery) Error() string {
	return gotext.Get("no query was executed")
}

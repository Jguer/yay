package topo

import "errors"

var (
	ErrSelfReferential  = errors.New("self-referential dependencies not allowed")
	ErrConflictingAlias = errors.New("alias already defined")
	ErrCircular         = errors.New("circular dependencies not allowed")
)

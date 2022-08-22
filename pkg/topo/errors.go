package topo

import "errors"

var ErrSelfReferential = errors.New("self-referential dependencies not allowed")
var ErrConflictingAlias = errors.New("alias already defined")
var ErrCircular = errors.New("circular dependencies not allowed")

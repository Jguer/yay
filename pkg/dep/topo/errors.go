package topo

import "errors"

var (
	// ErrSelfReferential is returned when attempting to create an edge where a node depends on itself.
	ErrSelfReferential = errors.New(" self-referential dependencies not allowed")

	// ErrConflictingAlias is reserved for when a "provides" alias is defined more than once.
	// Note: current Graph APIs overwrite provides entries; this error is not currently returned.
	ErrConflictingAlias = errors.New(" alias already defined")

	// ErrCircular is returned when attempting to create an edge that would introduce a cycle.
	ErrCircular = errors.New(" circular dependencies not allowed")
)

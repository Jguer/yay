package topo

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGraph_DependenciesAndDependents_Direct(t *testing.T) {
	g := New[string, struct{}]()

	// yay depends on go
	require.NoError(t, g.DependOn("yay", "go"))

	// Dependencies("yay") => {"go"}
	depsYay := g.Dependencies("yay")
	require.NotNil(t, depsYay)
	require.Len(t, depsYay, 1)
	require.True(t, depsYay["go"])

	// Dependencies("go") => {} (empty set, because "go" is in the graph but has no deps)
	depsGo := g.Dependencies("go")
	require.NotNil(t, depsGo)
	require.Len(t, depsGo, 0)

	// Dependents("go") => {"yay"}
	dependentsGo := g.Dependents("go")
	require.NotNil(t, dependentsGo)
	require.Len(t, dependentsGo, 1)
	require.True(t, dependentsGo["yay"])

	// Dependents("yay") => {} (empty set, because nothing depends on yay)
	dependentsYay := g.Dependents("yay")
	require.NotNil(t, dependentsYay)
	require.Len(t, dependentsYay, 0)
}

func TestGraph_DependenciesAndDependents_Transitive(t *testing.T) {
	g := New[string, struct{}]()

	// yay depends on go; foo depends on yay
	require.NoError(t, g.DependOn("yay", "go"))
	require.NoError(t, g.DependOn("foo", "yay"))

	// Dependencies("foo") => {"yay", "go"}
	depsFoo := g.Dependencies("foo")
	require.NotNil(t, depsFoo)
	require.Len(t, depsFoo, 2)
	require.True(t, depsFoo["yay"])
	require.True(t, depsFoo["go"])

	// Dependents("go") => {"yay", "foo"} (transitive)
	dependentsGo := g.Dependents("go")
	require.NotNil(t, dependentsGo)
	require.Len(t, dependentsGo, 2)
	require.True(t, dependentsGo["yay"])
	require.True(t, dependentsGo["foo"])

	// Dependents("yay") => {"foo"}
	dependentsYay := g.Dependents("yay")
	require.NotNil(t, dependentsYay)
	require.Len(t, dependentsYay, 1)
	require.True(t, dependentsYay["foo"])
}

func TestGraph_DependenciesAndDependents_MissingNodeReturnsNil(t *testing.T) {
	g := New[string, struct{}]()

	// For nodes not present in the graph, transitive queries return nil.
	require.Nil(t, g.Dependencies("missing"))
	require.Nil(t, g.Dependents("missing"))

	// Adding edges adds nodes; existing nodes with no deps/dependents return an empty set (non-nil).
	require.NoError(t, g.DependOn("yay", "go"))
	require.NotNil(t, g.Dependencies("go"))
	require.Len(t, g.Dependencies("go"), 0)
	require.NotNil(t, g.Dependents("yay"))
	require.Len(t, g.Dependents("yay"), 0)
}

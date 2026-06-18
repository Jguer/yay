//go:build !integration

package topo

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGraph_AddNodeAndLenAndExists(t *testing.T) {
	t.Parallel()

	graph := New[string, struct{}]()
	graph.AddNode("core")
	graph.AddNode("extra")
	graph.AddNode("core")

	require.Equal(t, 2, graph.Len())
	require.True(t, graph.Exists("core"))
	require.False(t, graph.Exists("missing"))
}

func TestGraph_DependOnRejectsSelfEdges(t *testing.T) {
	t.Parallel()

	graph := New[string, struct{}]()
	require.EqualError(t, ErrSelfReferential, graph.DependOn("a", "a").Error())

	// DependOn no longer detects cycles eagerly (O(V) per edge); cycles are
	// detected at sort time via HasCycle / TopoSortedLayers.
	require.NoError(t, graph.DependOn("a", "b"))
	require.NoError(t, graph.DependOn("b", "a"))
	require.NoError(t, graph.DependOn("c", "b"))
	require.True(t, graph.HasCycle())
}

func TestGraph_HasCycle(t *testing.T) {
	t.Parallel()

	t.Run("acyclic", func(t *testing.T) {
		t.Parallel()
		graph := New[string, struct{}]()
		require.NoError(t, graph.DependOn("yay", "go"))
		require.NoError(t, graph.DependOn("foo", "yay"))
		require.False(t, graph.HasCycle())
		require.Len(t, graph.TopoSortedLayers(nil), 3)
	})

	t.Run("two-node cycle", func(t *testing.T) {
		t.Parallel()
		graph := New[string, struct{}]()
		require.NoError(t, graph.DependOn("a", "b"))
		require.NoError(t, graph.DependOn("b", "a"))
		require.True(t, graph.HasCycle())
		// Nodes in the cycle are never emitted; only dependency-free nodes outside
		// the cycle (none here) would appear.
		require.Empty(t, graph.TopoSortedLayers(nil))
	})

	t.Run("self-loop rejected by DependOn not by HasCycle", func(t *testing.T) {
		t.Parallel()
		graph := New[string, struct{}]()
		require.EqualError(t, ErrSelfReferential, graph.DependOn("a", "a").Error())
	})
}

func TestGraph_CyclicNodes(t *testing.T) {
	t.Parallel()

	t.Run("acyclic returns empty", func(t *testing.T) {
		t.Parallel()
		graph := New[string, struct{}]()
		require.NoError(t, graph.DependOn("yay", "go"))
		require.NoError(t, graph.DependOn("foo", "yay"))
		require.Empty(t, graph.CyclicNodes())
		require.False(t, graph.HasCycle())
	})

	t.Run("two-node cycle names both nodes", func(t *testing.T) {
		t.Parallel()
		graph := New[string, struct{}]()
		require.NoError(t, graph.DependOn("a", "b"))
		require.NoError(t, graph.DependOn("b", "a"))
		cyclic := graph.CyclicNodes()
		require.Len(t, cyclic, 2)
		set := map[string]bool{}
		for _, n := range cyclic {
			set[n] = true
		}
		require.True(t, set["a"])
		require.True(t, set["b"])
		require.True(t, graph.HasCycle())
	})

	t.Run("cycle plus acyclic nodes names only cycle", func(t *testing.T) {
		t.Parallel()
		graph := New[string, struct{}]()
		// acyclic chain: foo -> yay -> go
		require.NoError(t, graph.DependOn("yay", "go"))
		require.NoError(t, graph.DependOn("foo", "yay"))
		// cycle: a -> b -> a
		require.NoError(t, graph.DependOn("a", "b"))
		require.NoError(t, graph.DependOn("b", "a"))
		cyclic := graph.CyclicNodes()
		require.Len(t, cyclic, 2)
		// the acyclic chain must still sort fully
		layers := graph.TopoSortedLayers(nil)
		emitted := 0
		for _, layer := range layers {
			emitted += len(layer)
		}
		require.Equal(t, 3, emitted, "acyclic nodes must still be emitted")
	})
}

func TestGraph_ForEachAndForEachError(t *testing.T) {
	t.Parallel()

	graph := New[string, int]()
	graph.AddNode("one")
	graph.SetNodeInfo("one", &NodeInfo[int]{Value: 1})
	graph.AddNode("two")
	graph.SetNodeInfo("two", &NodeInfo[int]{Value: 2})

	var seen []string
	err := graph.ForEach(func(node string, value int) error {
		seen = append(seen, node)
		return nil
	})
	require.NoError(t, err)
	require.Len(t, seen, 2)

	err = graph.ForEach(func(node string, value int) error {
		if node == "one" {
			return errors.New("stop")
		}

		return nil
	})
	require.EqualError(t, err, "stop")
}

func TestGraph_TopoSortedLayers_WithCheckFn(t *testing.T) {
	t.Parallel()

	graph := New[string, int]()
	graph.AddNode("root")
	graph.AddNode("leaf")
	require.NoError(t, graph.DependOn("leaf", "root"))

	called := 0
	layers := graph.TopoSortedLayers(func(node string, value int) error {
		called++
		return nil
	})
	require.Equal(t, 2, called)
	require.Len(t, layers, 2)
	require.Equal(t, map[string]int{"root": 0}, layers[0])

	called = 0
	layers = graph.TopoSortedLayers(func(node string, value int) error {
		called++

		if strings.Contains(node, "leaf") {
			return errors.New("halt")
		}

		return nil
	})
	require.Nil(t, layers)
	require.Equal(t, 2, called)
}

func TestGraph_PrunedNodes(t *testing.T) {
	t.Parallel()

	graph := New[string, int]()
	require.NoError(t, graph.DependOn("a", "b"))
	require.NoError(t, graph.DependOn("c", "a"))

	pruned := graph.Prune("a")
	require.Len(t, pruned, 3)
	require.Equal(t, 0, graph.Len())
	require.False(t, graph.Exists("a"))
	require.False(t, graph.Exists("b"))
	require.False(t, graph.Exists("c"))

	set := make(map[string]struct{}, len(pruned))
	for _, node := range pruned {
		set[node] = struct{}{}
	}
	require.Contains(t, set, "a")
	require.Contains(t, set, "b")
	require.Contains(t, set, "c")
}

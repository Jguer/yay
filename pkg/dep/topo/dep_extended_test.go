//go:build !integration
// +build !integration

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

func TestGraph_DependOnRejectsInvalidEdges(t *testing.T) {
	t.Parallel()

	graph := New[string, struct{}]()
	require.EqualError(t, ErrSelfReferential, graph.DependOn("a", "a").Error())

	require.NoError(t, graph.DependOn("a", "b"))
	require.EqualError(t, ErrCircular, graph.DependOn("b", "a").Error())
	require.NoError(t, graph.DependOn("c", "b"))
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

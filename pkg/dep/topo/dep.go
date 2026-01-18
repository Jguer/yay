package topo

import (
	"fmt"
	"maps"
	"strings"

	alpm "github.com/Jguer/dyalpm"
)

type (
	// NodeSet is a set of nodes represented as a map for O(1) membership checks.
	// The boolean value is not meaningful; presence of the key indicates membership.
	NodeSet[T comparable] map[T]bool

	// ProvidesMap maps a "provides" key (an alias/satisfier name) to information about the
	// node that provides it.
	ProvidesMap[T comparable] map[T]*DependencyInfo[T]

	// DepMap maps a node to a set of nodes. In Graph this is used for adjacency:
	// - dependencies: node -> its direct dependencies
	// - dependents:   node -> its direct dependents
	DepMap[T comparable] map[T]NodeSet[T]
)

// Slice returns the set contents as a slice in unspecified order.
func (n NodeSet[T]) Slice() []T {
	var slice []T

	for node := range n {
		slice = append(slice, node)
	}

	return slice
}

// NodeInfo carries optional rendering metadata (Color/Background) plus the node's value.
type NodeInfo[V any] struct {
	Color      string
	Background string
	Value      V
}

// DependencyInfo describes which node provides a given dependency/satisfier, along with the
// original dependency metadata (alpm.Depend).
type DependencyInfo[T comparable] struct {
	Provider T
	alpm.Depend
}

// CheckFn is a callback used by traversal helpers. It receives the node id plus the node's value.
type CheckFn[T comparable, V any] func(T, V) error

// Graph is a directed dependency graph.
//
// Edge direction:
// - An edge is added with DependOn(child, parent), meaning "child depends on parent".
// - Internally, dependencies maps child -> parents (direct dependencies).
// - Internally, dependents maps parent -> children (direct dependents).
type Graph[T comparable, V any] struct {
	nodes NodeSet[T]

	// node info map
	nodeInfo map[T]*NodeInfo[V]

	// `provides` tracks provides -> node.
	provides ProvidesMap[T]

	// `dependencies` tracks child -> parents.
	dependencies DepMap[T]
	// `dependents` tracks parent -> children.
	dependents DepMap[T]
}

// New returns an empty Graph.
func New[T comparable, V any]() *Graph[T, V] {
	return &Graph[T, V]{
		nodes:        make(NodeSet[T]),
		dependencies: make(DepMap[T]),
		dependents:   make(DepMap[T]),
		nodeInfo:     make(map[T]*NodeInfo[V]),
		provides:     make(ProvidesMap[T]),
	}
}

// Len returns the number of nodes currently present in the graph.
func (g *Graph[T, V]) Len() int {
	return len(g.nodes)
}

// Exists reports whether node exists in the graph's node set.
func (g *Graph[T, V]) Exists(node T) bool {
	_, ok := g.nodes[node]

	return ok
}

// AddNode adds node to the graph. It is safe to call multiple times.
func (g *Graph[T, V]) AddNode(node T) {
	g.nodes[node] = true
}

// HasProvides reports whether the given provides key is registered.
func (g *Graph[T, V]) HasProvides(provides T) bool {
	_, ok := g.provides[provides]

	return ok
}

// GetProviderInfo returns the dependency info for a provider.
func (g *Graph[T, V]) GetProviderInfo(provides T) *DependencyInfo[T] {
	return g.provides[provides]
}

// AddProvides registers that node provides the given provides key.
//
// Note: despite the "Add" name, this is a single mapping; calling it again with the same
// provides key overwrites the previous entry.
func (g *Graph[T, V]) AddProvides(provides T, depInfo *alpm.Depend, node T) {
	g.provides[provides] = &DependencyInfo[T]{
		Provider: node,
		Depend:   *depInfo,
	}
}

// ForEach calls f for every node in the graph.
//
// The value passed to f is the node's NodeInfo.Value if set via SetNodeInfo; otherwise it is
// the zero value of V.
func (g *Graph[T, V]) ForEach(f CheckFn[T, V]) error {
	for node := range g.nodes {
		var v V
		if info := g.nodeInfo[node]; info != nil {
			v = info.Value
		}
		if err := f(node, v); err != nil {
			return err
		}
	}

	return nil
}

// SetNodeInfo sets metadata and value for node. Node does not need to already exist in the graph.
func (g *Graph[T, V]) SetNodeInfo(node T, nodeInfo *NodeInfo[V]) {
	g.nodeInfo[node] = nodeInfo
}

// GetNodeInfo returns metadata/value for node, or nil if none was set.
func (g *Graph[T, V]) GetNodeInfo(node T) *NodeInfo[V] {
	return g.nodeInfo[node]
}

// DependOn adds an edge meaning "child depends on parent".
//
// This ensures both nodes exist in the graph and rejects:
// - self edges (ErrSelfReferential)
// - edges that would introduce a cycle (ErrCircular)
func (g *Graph[T, V]) DependOn(child, parent T) error {
	if child == parent {
		return ErrSelfReferential
	}

	if g.DependsOn(parent, child) {
		return ErrCircular
	}

	g.AddNode(parent)
	g.AddNode(child)

	// Add edges.
	g.dependents.add(parent, child)
	g.dependencies.add(child, parent)

	return nil
}

// String renders the graph in GraphViz DOT format.
//
// Nodes are emitted as `"node"` entries with optional color metadata from NodeInfo.
// Edges are emitted in the direction: dependent -> dependency (child -> parent).
func (g *Graph[T, V]) String() string {
	var sb strings.Builder

	sb.WriteString("digraph {\n")
	sb.WriteString("compound=true;\n")
	sb.WriteString("concentrate=true;\n")
	sb.WriteString("node [shape = record, ordering=out];\n")

	for node := range g.nodes {
		extra := ""

		if info, ok := g.nodeInfo[node]; ok {
			if info.Background != "" || info.Color != "" {
				extra = fmt.Sprintf("[color = %s, style = filled, fillcolor = %s]", info.Color, info.Background)
			}
		}

		sb.WriteString(fmt.Sprintf("\t\"%v\"%s;\n", node, extra))
	}

	for parent, children := range g.dependencies {
		for child := range children {
			sb.WriteString(fmt.Sprintf("\t\"%v\" -> \"%v\";\n", parent, child))
		}
	}

	sb.WriteString("}")

	return sb.String()
}

// DependsOn reports whether child depends (transitively) on parent.
func (g *Graph[T, V]) DependsOn(child, parent T) bool {
	deps := g.Dependencies(child)
	_, ok := deps[parent]

	return ok
}

// HasDependent reports whether parent has dependent as a (transitive) dependent.
func (g *Graph[T, V]) HasDependent(parent, dependent T) bool {
	deps := g.Dependents(parent)
	_, ok := deps[dependent]

	return ok
}

// leavesMap returns a map of leaves with the node as key and the node info value as value.
func (g *Graph[T, V]) leavesMap() map[T]V {
	leaves := make(map[T]V, 0)

	for node := range g.nodes {
		if _, ok := g.dependencies[node]; !ok {
			nodeInfo := g.GetNodeInfo(node)
			if nodeInfo == nil {
				nodeInfo = &NodeInfo[V]{}
			}

			leaves[node] = nodeInfo.Value
		}
	}

	return leaves
}

// TopoSortedLayers returns a slice of all of the graph nodes in topological sort order with their node info.
//
// The returned slice is layered: each element is a "layer" of nodes that have no remaining
// dependencies at that stage of the process.
//
// Practical meaning with this graph's edge direction (DependOn(child, parent)):
// - Earlier layers contain nodes with fewer/zero dependencies (i.e. dependencies-first order).
// - A node appears only after all of its dependencies have appeared in earlier layers.
//
// If checkFn is non-nil, it is called once per node when it is emitted in a layer. Returning an
// error causes TopoSortedLayers to return nil.
func (g *Graph[T, V]) TopoSortedLayers(checkFn CheckFn[T, V]) []map[T]V {
	layers := []map[T]V{}

	// Copy the graph
	shrinkingGraph := g.clone()

	for {
		leaves := shrinkingGraph.leavesMap()
		if len(leaves) == 0 {
			break
		}

		layers = append(layers, leaves)

		for leafNode := range leaves {
			if checkFn != nil {
				if err := checkFn(leafNode, leaves[leafNode]); err != nil {
					return nil
				}
			}
			shrinkingGraph.remove(leafNode)
		}
	}

	return layers
}

// returns if it was the last
func (dm DepMap[T]) remove(key, node T) bool {
	if nodes := dm[key]; len(nodes) == 1 {
		// The only element in the nodeset must be `node`, so we
		// can delete the entry entirely.
		delete(dm, key)
		return true
	} else {
		// Otherwise, remove the single node from the nodeset.
		delete(nodes, node)
		return false
	}
}

// Prune removes the node,
// its dependencies if there are no other dependents
// and its dependents
//
// It returns the list of nodes that were removed (including node). The returned order is based
// on recursive traversal and is not guaranteed to be stable.
func (g *Graph[T, V]) Prune(node T) []T {
	pruned := []T{node}
	// Remove edges from things that depend on `node`.
	for dependent := range g.dependents[node] {
		last := g.dependencies.remove(dependent, node)
		if last {
			pruned = append(pruned, g.Prune(dependent)...)
		}
	}

	delete(g.dependents, node)

	// Remove all edges from node to the things it depends on.
	for dependency := range g.dependencies[node] {
		last := g.dependents.remove(dependency, node)
		if last {
			pruned = append(pruned, g.Prune(dependency)...)
		}
	}

	delete(g.dependencies, node)

	// Finally, remove the node itself.
	delete(g.nodes, node)
	return pruned
}

func (g *Graph[T, V]) remove(node T) {
	// Remove edges from things that depend on `node`.
	for dependent := range g.dependents[node] {
		g.dependencies.remove(dependent, node)
	}

	delete(g.dependents, node)

	// Remove all edges from node to the things it depends on.
	for dependency := range g.dependencies[node] {
		g.dependents.remove(dependency, node)
	}

	delete(g.dependencies, node)

	// Finally, remove the node itself.
	delete(g.nodes, node)
}

// Dependencies returns all transitive dependencies of child (excluding child itself).
// The returned set is nil if child is not present in the graph.
func (g *Graph[T, V]) Dependencies(child T) NodeSet[T] {
	return g.buildTransitive(child, g.ImmediateDependencies)
}

// ImmediateDependencies returns the direct dependencies of node.
// The returned set is nil if node has no direct dependencies (or is not present).
func (g *Graph[T, V]) ImmediateDependencies(node T) NodeSet[T] {
	return g.dependencies[node]
}

// Dependents returns all transitive dependents of parent (excluding parent itself).
// The returned set is nil if parent is not present in the graph.
func (g *Graph[T, V]) Dependents(parent T) NodeSet[T] {
	return g.buildTransitive(parent, g.ImmediateDependents)
}

// ImmediateDependents returns the direct dependents of node.
// The returned set is nil if node has no direct dependents (or is not present).
func (g *Graph[T, V]) ImmediateDependents(node T) NodeSet[T] {
	return g.dependents[node]
}

func (g *Graph[T, V]) clone() *Graph[T, V] {
	return &Graph[T, V]{
		dependencies: g.dependencies.copy(),
		dependents:   g.dependents.copy(),
		nodes:        g.nodes.copy(),
		nodeInfo:     g.nodeInfo, // not copied, as it is not modified
	}
}

// buildTransitive starts at `root` and continues calling `nextFn` to keep discovering more nodes until
// the graph cannot produce any more. It returns the set of all discovered nodes.
func (g *Graph[T, V]) buildTransitive(root T, nextFn func(T) NodeSet[T]) NodeSet[T] {
	if _, ok := g.nodes[root]; !ok {
		return nil
	}

	out := make(NodeSet[T])
	searchNext := []T{root}

	for len(searchNext) > 0 {
		// List of new nodes from this layer of the dependency graph. This is
		// assigned to `searchNext` at the end of the outer "discovery" loop.
		discovered := []T{}

		for _, node := range searchNext {
			// For each node to discover, find the next nodes.
			for nextNode := range nextFn(node) {
				// If we have not seen the node before, add it to the output as well
				// as the list of nodes to traverse in the next iteration.
				if _, ok := out[nextNode]; !ok {
					out[nextNode] = true

					discovered = append(discovered, nextNode)
				}
			}
		}

		searchNext = discovered
	}

	return out
}

func (s NodeSet[T]) copy() NodeSet[T] {
	out := make(NodeSet[T], len(s))
	maps.Copy(out, s)

	return out
}

func (dm DepMap[T]) copy() DepMap[T] {
	out := make(DepMap[T], len(dm))
	for k := range dm {
		out[k] = dm[k].copy()
	}

	return out
}

func (dm DepMap[T]) add(key, node T) {
	nodes, ok := dm[key]
	if !ok {
		nodes = make(NodeSet[T])
		dm[key] = nodes
	}

	nodes[node] = true
}

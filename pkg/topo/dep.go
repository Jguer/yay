package topo

import (
	"fmt"
	"strings"
)

type Mapable interface {
	Key() string
}

type (
	AliasMap[T comparable] map[T]T
	NodeSet[T comparable]  map[T]bool
	DepMap[T comparable]   map[T]NodeSet[T]
)

type NodeInfo[V any] struct {
	Color      string
	Background string
	Value      V
}

type Graph[T comparable, V any] struct {
	alias AliasMap[T]
	nodes NodeSet[T]

	// node info map
	nodeInfo map[T]*NodeInfo[V]

	// `dependencies` tracks child -> parents.
	dependencies DepMap[T]
	// `dependents` tracks parent -> children.
	dependents DepMap[T]
	// Keep track of the nodes of the graph themselves.
}

func New[T comparable, V any]() *Graph[T, V] {
	return &Graph[T, V]{
		nodes:        make(NodeSet[T]),
		dependencies: make(DepMap[T]),
		dependents:   make(DepMap[T]),
		alias:        make(AliasMap[T]),
		nodeInfo:     make(map[T]*NodeInfo[V]),
	}
}

func (g *Graph[T, V]) Len() int {
	return len(g.nodes)
}

func (g *Graph[T, V]) Exists(node T) bool {
	// check aliases
	node = g.getAlias(node)

	_, ok := g.nodes[node]

	return ok
}

func (g *Graph[T, V]) Alias(node, alias T) error {
	if alias == node {
		return nil
	}

	// add node
	g.nodes[node] = true

	// add alias
	if _, ok := g.alias[alias]; ok {
		return ErrConflictingAlias
	}
	g.alias[alias] = node

	return nil
}

func (g *Graph[T, V]) AddNode(node T) {
	node = g.getAlias(node)

	g.nodes[node] = true
}

func (g *Graph[T, V]) getAlias(node T) T {
	if aliasNode, ok := g.alias[node]; ok {
		return aliasNode
	}
	return node
}

func (g *Graph[T, V]) SetNodeInfo(node T, nodeInfo *NodeInfo[V]) {
	node = g.getAlias(node)

	g.nodeInfo[node] = nodeInfo
}

func (g *Graph[T, V]) GetNodeInfo(node T) *NodeInfo[V] {
	node = g.getAlias(node)

	return g.nodeInfo[node]
}

func (g *Graph[T, V]) DependOn(child, parent T) error {
	child = g.getAlias(child)
	parent = g.getAlias(parent)

	if child == parent {
		return ErrSelfReferential
	}

	if g.DependsOn(parent, child) {
		return ErrCircular
	}

	g.AddNode(parent)
	g.AddNode(child)

	// Add nodes.
	g.nodes[parent] = true
	g.nodes[child] = true

	// Add edges.
	g.dependents.addNodeToNodeset(parent, child)
	g.dependencies.addNodeToNodeset(child, parent)

	return nil
}

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
				extra = fmt.Sprintf("[color = %s ,style = filled, fillcolor = %s]", info.Color, info.Background)
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

func (g *Graph[T, V]) DependsOn(child, parent T) bool {
	deps := g.Dependencies(child)
	_, ok := deps[parent]

	return ok
}

func (g *Graph[T, V]) HasDependent(parent, child T) bool {
	deps := g.Dependents(parent)
	_, ok := deps[child]

	return ok
}

func (g *Graph[T, V]) Leaves() []T {
	leaves := make([]T, 0)

	for node := range g.nodes {
		if _, ok := g.dependencies[node]; !ok {
			leaves = append(leaves, node)
		}
	}

	return leaves
}

// TopoSortedLayers returns a slice of all of the graph nodes in topological sort order. That is,
// if `B` depends on `A`, then `A` is guaranteed to come before `B` in the sorted output.
// The graph is guaranteed to be cycle-free because cycles are detected while building the
// graph. Additionally, the output is grouped into "layers", which are guaranteed to not have
// any dependencies within each layer. This is useful, e.g. when building an execution plan for
// some DAG, in which case each element within each layer could be executed in parallel. If you
// do not need this layered property, use `Graph.TopoSorted()`, which flattens all elements.
func (g *Graph[T, V]) TopoSortedLayers() [][]T {
	layers := [][]T{}

	// Copy the graph
	shrinkingGraph := g.clone()

	for {
		leaves := shrinkingGraph.Leaves()
		if len(leaves) == 0 {
			break
		}

		layers = append(layers, leaves)

		for _, leafNode := range leaves {
			shrinkingGraph.remove(leafNode)
		}
	}

	return layers
}

func (dm DepMap[T]) removeFromDepmap(key, node T) {
	if nodes := dm[key]; len(nodes) == 1 {
		// The only element in the nodeset must be `node`, so we
		// can delete the entry entirely.
		delete(dm, key)
	} else {
		// Otherwise, remove the single node from the nodeset.
		delete(nodes, node)
	}
}

func (g *Graph[T, V]) remove(node T) {
	// Remove edges from things that depend on `node`.
	for dependent := range g.dependents[node] {
		g.dependencies.removeFromDepmap(dependent, node)
	}

	delete(g.dependents, node)

	// Remove all edges from node to the things it depends on.
	for dependency := range g.dependencies[node] {
		g.dependents.removeFromDepmap(dependency, node)
	}

	delete(g.dependencies, node)

	// Finally, remove the node itself.
	delete(g.nodes, node)
}

// TopoSorted returns all the nodes in the graph is topological sort order.
// See also `Graph.TopoSortedLayers()`.
func (g *Graph[T, V]) TopoSorted() []T {
	nodeCount := 0
	layers := g.TopoSortedLayers()

	for _, layer := range layers {
		nodeCount += len(layer)
	}

	allNodes := make([]T, 0, nodeCount)

	for _, layer := range layers {
		allNodes = append(allNodes, layer...)
	}

	return allNodes
}

func (g *Graph[T, V]) Dependencies(child T) NodeSet[T] {
	return g.buildTransitive(child, g.immediateDependencies)
}

func (g *Graph[T, V]) immediateDependencies(node T) NodeSet[T] {
	return g.dependencies[node]
}

func (g *Graph[T, V]) Dependents(parent T) NodeSet[T] {
	return g.buildTransitive(parent, g.immediateDependents)
}

func (g *Graph[T, V]) immediateDependents(node T) NodeSet[T] {
	return g.dependents[node]
}

func (g *Graph[T, V]) clone() *Graph[T, V] {
	return &Graph[T, V]{
		dependencies: g.dependencies.copy(),
		dependents:   g.dependents.copy(),
		nodes:        g.nodes.copy(),
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
	for k, v := range s {
		out[k] = v
	}

	return out
}

func (m DepMap[T]) copy() DepMap[T] {
	out := make(DepMap[T], len(m))
	for k, v := range m {
		out[k] = v.copy()
	}

	return out
}

func (dm DepMap[T]) addNodeToNodeset(key, node T) {
	nodes, ok := dm[key]
	if !ok {
		nodes = make(NodeSet[T])
		dm[key] = nodes
	}

	nodes[node] = true
}

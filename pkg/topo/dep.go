package topo

import (
	"fmt"
	"strings"
)

type (
	NodeSet[T comparable] map[T]bool
	DepMap[T comparable]  map[T]NodeSet[T]
)

type NodeInfo[V any] struct {
	Color      string
	Background string
	Value      V
}

type CheckFn[T comparable, V any] func(T, V) error

type Graph[T comparable, V any] struct {
	nodes NodeSet[T]

	// node info map
	nodeInfo map[T]*NodeInfo[V]

	// `dependencies` tracks child -> parents.
	dependencies DepMap[T]
	// `dependents` tracks parent -> children.
	dependents DepMap[T]
}

func New[T comparable, V any]() *Graph[T, V] {
	return &Graph[T, V]{
		nodes:        make(NodeSet[T]),
		dependencies: make(DepMap[T]),
		dependents:   make(DepMap[T]),
		nodeInfo:     make(map[T]*NodeInfo[V]),
	}
}

func (g *Graph[T, V]) Exists(node T) bool {
	_, ok := g.nodes[node]

	return ok
}

func (g *Graph[T, V]) AddNode(node T) {
	g.nodes[node] = true
}

func (g *Graph[T, V]) ForEach(f CheckFn[T, V]) error {
	for node := range g.nodes {
		if err := f(node, g.nodeInfo[node].Value); err != nil {
			return err
		}
	}

	return nil
}

func (g *Graph[T, V]) SetNodeInfo(node T, nodeInfo *NodeInfo[V]) {
	g.nodeInfo[node] = nodeInfo
}

func (g *Graph[T, V]) GetNodeInfo(node T) *NodeInfo[V] {
	return g.nodeInfo[node]
}

func (g *Graph[T, V]) DependOn(child, parent T) error {
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

// TopoSortedLayerMap returns a slice of all of the graph nodes in topological sort order with their node info.
func (g *Graph[T, V]) TopoSortedLayerMap(checkFn CheckFn[T, V]) []map[T]V {
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
	for k, v := range s {
		out[k] = v
	}

	return out
}

func (dm DepMap[T]) copy() DepMap[T] {
	out := make(DepMap[T], len(dm))
	for k := range dm {
		out[k] = dm[k].copy()
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

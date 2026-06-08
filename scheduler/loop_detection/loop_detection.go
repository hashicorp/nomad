// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package loop_detection

import (
	"fmt"
	"sort"
	"sync"

	log "github.com/hashicorp/go-hclog"
)

// WIP
// Loop Detection + Dependency Chain Update Flow
//
// [Start: incoming dependency edge job -> dependee]
//                    |
//                    v
//      [Ensure nodes exist in graph index/map]
//                    |
//                    v
//   [Check whether job or dependee already has chains]
//                    |
//                    v
//      [Build/lookup candidate chain path(s)]
//                    |
//                    v
//         [Run cycle/loop detection on path]
//                    |
//          +---------+---------+
//          |                   |
//          v                   v
//   [Loop found? YES]     [Loop found? NO]
//          |                   |
//          v                   v
// [Return error/reject]   [Add edge job -> dependee]
//                              |
//                              v
//              [Update node->chains mapping/index]
//                              |
//                              v
//               [Dependency accepted / continue]
//
// -------------------------------------------------------
//
// [Event: job becomes unblocked / completed / removed]
//                    |
//                    v
//      [Remove job node and its outgoing/incoming edges]
//                    |
//                    v
//        [Recompute/prune affected dependency chains]
//                    |
//                    v
//          [Update node->chains mapping/index]
//                    |
//                    v
//                     [Done]

// Detector tracks a directed dependency graph and maintains a chains index
// for each node. Edges are directed as "job -> dependee".
type Detector struct {
	logger log.Logger

	mu sync.RWMutex

	// deps stores outgoing edges: job -> dependees.
	deps map[string]map[string]struct{}

	// reverse stores incoming edges: dependee -> jobs that depend on it.
	reverse map[string]map[string]struct{}

	// chains stores all root-to-leaf dependency chains for each node.
	chains map[string][][]string

	// nodes stores pointers to graph nodes by job id.
	nodes map[string]*Node

	// graphs stores dependency chain paths as arrays of node pointers.
	graphs [][]*Node
}

// Node represents a graph node and its outgoing dependencies.
type Node struct {
	ID           string
	Dependencies []string
}

// NewDetector creates an empty dependency detector.
func NewDetector(logger log.Logger) *Detector {
	return &Detector{
		logger:  logger.Named("loop_detector"),
		deps:    make(map[string]map[string]struct{}),
		reverse: make(map[string]map[string]struct{}),
		chains:  make(map[string][][]string),
		nodes:   make(map[string]*Node),
		graphs:  make([][]*Node, 0),
	}
}

func (d *Detector) AddNodes(nodeID string, dependencies ...string) error {
	if err := d.addNode(Node{ID: nodeID, Dependencies: dependencies}); err != nil {
		return fmt.Errorf("failed to add node %q: %v", nodeID, err)
	}
	return nil
}

// AddNode ensures node exists and adds edges node.ID -> dependency for each
// listed dependency.
//
// If any dependency would create a loop, AddNode returns an error and the
// graph remains unchanged.
func (d *Detector) addNode(node Node) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if node.ID == "" {
		return fmt.Errorf("node id must be non-empty")
	}

	d.ensureNode(node.ID)

	uniqueDeps := make(map[string]struct{}, len(node.Dependencies))
	for _, dep := range node.Dependencies {
		if dep == "" {
			return fmt.Errorf("dependency for %q must be non-empty", node.ID)
		}
		if dep == node.ID {
			return fmt.Errorf("self-dependency is not allowed for %q", node.ID)
		}
		uniqueDeps[dep] = struct{}{}
	}

	// Validate all candidate edges first so updates are all-or-nothing.
	for dep := range uniqueDeps {
		d.ensureNode(dep)
		if _, exists := d.deps[node.ID][dep]; exists {
			continue
		}
		if d.pathExists(dep, node.ID) {
			return fmt.Errorf("adding dependency %q -> %q creates a loop", node.ID, dep)
		}
	}

	for dep := range uniqueDeps {
		d.deps[node.ID][dep] = struct{}{}
		d.reverse[dep][node.ID] = struct{}{}
	}

	d.recomputeChainsLocked()
	d.syncNodeIndexesLocked()

	return nil
}

// RemoveNode removes the target node and its direct dependees (one hop only).
// Removal is non-cascading.
func (d *Detector) RemoveNode(node string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if node == "" {
		return fmt.Errorf("node id must be non-empty")
	}

	if _, exists := d.deps[node]; !exists {
		return nil
	}

	directDependees := make([]string, 0, len(d.deps[node]))
	for dependee := range d.deps[node] {
		directDependees = append(directDependees, dependee)
	}

	d.removeSingleNodeLocked(node)
	for _, dependee := range directDependees {
		d.removeSingleNodeLocked(dependee)
	}

	d.recomputeChainsLocked()
	d.syncNodeIndexesLocked()

	return nil
}

func (d *Detector) ensureNode(node string) {
	if _, ok := d.deps[node]; !ok {
		d.deps[node] = make(map[string]struct{})
	}
	if _, ok := d.reverse[node]; !ok {
		d.reverse[node] = make(map[string]struct{})
	}
	if _, ok := d.nodes[node]; !ok {
		d.nodes[node] = &Node{ID: node}
	}
}

func (d *Detector) pathExists(from, to string) bool {
	if from == to {
		return true
	}

	seen := make(map[string]struct{})
	stack := []string{from}

	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if n == to {
			return true
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}

		for next := range d.deps[n] {
			if _, ok := seen[next]; !ok {
				stack = append(stack, next)
			}
		}
	}

	return false
}

func (d *Detector) recomputeChainsLocked() {
	allNodes := make(map[string]struct{}, len(d.deps)+len(d.reverse))
	for n := range d.deps {
		allNodes[n] = struct{}{}
	}
	for n := range d.reverse {
		allNodes[n] = struct{}{}
	}

	d.chains = make(map[string][][]string, len(allNodes))
	memo := make(map[string][][]string, len(allNodes))
	visiting := make(map[string]bool, len(allNodes))

	for n := range allNodes {
		d.chains[n] = d.buildChainsFromLocked(n, memo, visiting)
	}
}

func (d *Detector) removeSingleNodeLocked(node string) {
	// Remove outgoing edges: node -> dependee.
	for dependee := range d.deps[node] {
		delete(d.reverse[dependee], node)
		if len(d.reverse[dependee]) == 0 {
			delete(d.reverse, dependee)
		}
	}

	// Remove incoming edges: depender -> node.
	for depender := range d.reverse[node] {
		delete(d.deps[depender], node)
		if len(d.deps[depender]) == 0 {
			delete(d.deps, depender)
		}
	}

	delete(d.deps, node)
	delete(d.reverse, node)
	delete(d.chains, node)
	delete(d.nodes, node)
}

func (d *Detector) syncNodeIndexesLocked() {
	for id := range d.deps {
		d.ensureNode(id)
		deps := make([]string, 0, len(d.deps[id]))
		for dep := range d.deps[id] {
			deps = append(deps, dep)
		}
		sort.Strings(deps)
		d.nodes[id].Dependencies = deps
	}

	for id := range d.nodes {
		if _, ok := d.deps[id]; !ok {
			delete(d.nodes, id)
		}
	}

	graphs := make([][]*Node, 0)
	for _, paths := range d.chains {
		for _, path := range paths {
			nodePath := make([]*Node, 0, len(path))
			for _, id := range path {
				n, ok := d.nodes[id]
				if !ok {
					d.ensureNode(id)
					n = d.nodes[id]
				}
				nodePath = append(nodePath, n)
			}
			graphs = append(graphs, nodePath)
		}
	}
	d.graphs = graphs
}

func (d *Detector) buildChainsFromLocked(node string, memo map[string][][]string, visiting map[string]bool) [][]string {
	if cached, ok := memo[node]; ok {
		return copyPaths(cached)
	}

	if visiting[node] {
		// Guard for unexpected cycles. Cycles should not be reachable because
		// AddDependency rejects loop-inducing edges.
		return [][]string{{node}}
	}

	visiting[node] = true
	defer func() { visiting[node] = false }()

	nextNodes := make([]string, 0, len(d.deps[node]))
	for next := range d.deps[node] {
		nextNodes = append(nextNodes, next)
	}
	sort.Strings(nextNodes)

	if len(nextNodes) == 0 {
		memo[node] = [][]string{{node}}
		return copyPaths(memo[node])
	}

	paths := make([][]string, 0)
	for _, next := range nextNodes {
		subPaths := d.buildChainsFromLocked(next, memo, visiting)
		for _, sub := range subPaths {
			path := make([]string, 0, len(sub)+1)
			path = append(path, node)
			path = append(path, sub...)
			paths = append(paths, path)
		}
	}

	memo[node] = copyPaths(paths)
	return copyPaths(paths)
}

func copyPaths(paths [][]string) [][]string {
	out := make([][]string, len(paths))
	for i, p := range paths {
		cp := make([]string, len(p))
		copy(cp, p)
		out[i] = cp
	}
	return out
}

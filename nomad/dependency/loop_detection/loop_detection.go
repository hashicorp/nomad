// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package loop_detection

import (
	"errors"
	"fmt"
	"sync"

	"github.com/hashicorp/go-hclog"
)

var (
	ErrEmptyNodeID        = errors.New("node id cannot be empty")
	ErrSelfDependency     = errors.New("node cannot depend on itself")
	ErrNodeNotFound       = errors.New("node not found")
	ErrNodeIsDependency   = errors.New("cannot remove node: another node depends on it")
	ErrCircularDependency = errors.New("circular dependency detected")
)

// loopDetector maintains a directed dependency graph.
// deps:
//
//	node -> nodes it depends on
//
// dependents:
//
//	node -> nodes that depend on it
//
// Both maps also act as the node index, so node lookup is expected O(1).
type loopDetector struct {
	logger hclog.Logger
	mu     sync.RWMutex

	deps       map[string]map[string]struct{}
	dependents map[string]map[string]struct{}
}

// New creates an empty dependency graph.
func New(logger hclog.Logger) *loopDetector {
	return &loopDetector{
		logger:     logger.Named("loop-detection"),
		deps:       make(map[string]map[string]struct{}),
		dependents: make(map[string]map[string]struct{}),
	}
}

// AddNodes adds dependencies to nodeID.
// Before adding:
//
//	nodeID -> dependency
//
// we check whether dependency can already reach nodeID.
// If it can, adding the edge would create a cycle.
func (s *loopDetector) AddNodes(nodeID string, dependencies ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if nodeID == "" {
		return ErrEmptyNodeID
	}

	// Normalize and validate the complete request before modifying the graph.
	newDeps := make([]string, 0, len(dependencies))
	seen := make(map[string]struct{}, len(dependencies))

	for _, dep := range dependencies {
		if dep == "" {
			return ErrEmptyNodeID
		}

		if dep == nodeID {
			return ErrSelfDependency
		}

		if _, ok := seen[dep]; ok {
			continue
		}

		seen[dep] = struct{}{}
		newDeps = append(newDeps, dep)
	}

	// Validate all proposed edges before changing any state.
	//
	// Adding:
	//
	//     nodeID -> dep
	//
	// creates a cycle if dep can already reach nodeID.
	for _, dep := range newDeps {
		// Existing edges don't need to be checked again.
		if deps, ok := s.deps[nodeID]; ok {
			if _, exists := deps[dep]; exists {
				continue
			}
		}

		if s.reaches(dep, nodeID) {
			return fmt.Errorf("%w: %s -> %s would create a loop", ErrCircularDependency,
				nodeID, dep)
		}
	}

	// All validation succeeded. Now mutate the graph.
	s.ensureNode(nodeID)

	for _, dep := range newDeps {
		s.ensureNode(dep)

		// Edge may already exist.
		if _, exists := s.deps[nodeID][dep]; exists {
			continue
		}

		s.deps[nodeID][dep] = struct{}{}
		s.dependents[dep][nodeID] = struct{}{}
	}

	return nil
}

// RemoveNode removes nodeID if no other node depends on it.
//
// Complexity:
//
//   - node lookup: expected O(1)
//   - no dependencies: expected O(1)
//   - N dependencies: O(N)
//
// A node that is still referenced by another node is not removed.
func (s *loopDetector) RemoveNode(nodeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if nodeID == "" {
		return ErrEmptyNodeID
	}

	// Expected O(1) lookup.
	deps, ok := s.deps[nodeID]
	if !ok {
		return ErrNodeNotFound
	}

	// Expected O(1): len(map).
	if len(s.dependents[nodeID]) > 0 {
		return ErrNodeIsDependency
	}

	// Remove reverse references for outgoing edges.
	//
	// If the node has no dependencies, this loop performs zero iterations,
	// making removal expected O(1).
	children := make([]string, 0, len(deps))

	for dep := range deps {
		delete(s.dependents[dep], nodeID)
		children = append(children, dep)
	}

	delete(s.deps, nodeID)
	delete(s.dependents, nodeID)

	// Preserve the previous behavior of pruning dependency nodes that
	// become completely unreferenced.
	for _, child := range children {
		s.pruneOrphan(child)
	}

	return nil
}

// ensureNode initializes entries for nodeID.
//
// Caller must hold s.mu.
func (s *loopDetector) ensureNode(nodeID string) {
	if _, ok := s.deps[nodeID]; !ok {
		s.deps[nodeID] = make(map[string]struct{})
	}

	if _, ok := s.dependents[nodeID]; !ok {
		s.dependents[nodeID] = make(map[string]struct{})
	}
}

// reaches reports whether start can reach target by following dependency
// edges.
//
// It uses an iterative DFS to avoid recursion depth concerns.
//
// Caller must hold s.mu.
func (s *loopDetector) reaches(start, target string) bool {
	if start == target {
		return true
	}

	visited := make(map[string]struct{})
	stack := []string{start}

	for len(stack) > 0 {
		last := len(stack) - 1
		node := stack[last]
		stack = stack[:last]

		if _, ok := visited[node]; ok {
			continue
		}

		visited[node] = struct{}{}

		for dep := range s.deps[node] {
			if dep == target {
				return true
			}

			if _, seen := visited[dep]; !seen {
				stack = append(stack, dep)
			}
		}
	}

	return false
}

// pruneOrphan removes nodeID when:
//
//   - the node exists
//   - no active node depends on it
//
// It recursively removes any dependency nodes that become orphaned.
//
// Caller must hold s.mu.
func (s *loopDetector) pruneOrphan(nodeID string) {
	deps, ok := s.deps[nodeID]
	if !ok {
		return
	}

	if len(s.dependents[nodeID]) > 0 {
		return
	}

	children := make([]string, 0, len(deps))

	for child := range deps {
		delete(s.dependents[child], nodeID)
		children = append(children, child)
	}

	delete(s.deps, nodeID)
	delete(s.dependents, nodeID)

	for _, child := range children {
		s.pruneOrphan(child)
	}
}

// CreatesCircularDependency reports whether adding any of the edges
//
//	dep -> nodeID
//
// would create a circular dependency.
//
// A cycle exists if nodeID already reaches dep through the dependency graph.
func (s *loopDetector) CreatesCircularDependency(dep string, nodeIDs ...string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if dep == "" {
		return false
	}

	for _, nodeID := range nodeIDs {
		if nodeID == "" {
			continue
		}

		if nodeID == dep || s.reaches(nodeID, dep) {
			return true
		}
	}

	return false
}

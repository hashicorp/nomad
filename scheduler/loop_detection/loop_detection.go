package depgraph

import (
	"errors"
	"fmt"
	"sync"
)

var (
	ErrEmptyNodeID      = errors.New("node id cannot be empty")
	ErrSelfDependency   = errors.New("node cannot depend on itself")
	ErrNodeNotFound     = errors.New("node not found")
	ErrNodeIsDependency = errors.New("cannot remove node: another node depends on it")
)

// Graph is the only public interface.
type Graph interface {
	AddNodes(nodeID string, dependencies ...string) error
	RemoveNode(nodeID string) error
}

// Store implements Graph.
// Internally it keeps:
// 1) an array of linked lists
// 2) a map[nodeID]*linkedList
type Store struct {
	mu sync.RWMutex

	allLists []*linkedList
	byNode   map[string]*linkedList
	index    map[string]int // nodeID -> position in allLists

	// adjacency: node -> dependencies
	deps map[string]map[string]struct{}
	// reverse adjacency: node -> dependents
	dependents map[string]map[string]struct{}
}

type listNode struct {
	id   string
	next *listNode
}

type linkedList struct {
	head *listNode // head is the owner node
	tail *listNode
}

func newLinkedList(owner string) *linkedList {
	h := &listNode{id: owner}
	return &linkedList{head: h, tail: h}
}

func (l *linkedList) appendUnique(dep string) bool {
	if l.head == nil {
		l.head = &listNode{id: dep}
		l.tail = l.head
		return true
	}
	for n := l.head.next; n != nil; n = n.next {
		if n.id == dep {
			return false
		}
	}
	n := &listNode{id: dep}
	l.tail.next = n
	l.tail = n
	return true
}

// New creates an empty dependency graph.
func New() *Store {
	return &Store{
		allLists:   make([]*linkedList, 0),
		byNode:     make(map[string]*linkedList),
		index:      make(map[string]int),
		deps:       make(map[string]map[string]struct{}),
		dependents: make(map[string]map[string]struct{}),
	}
}

// AddNodes adds/updates nodeID with dependencies.
// It prevents circular dependencies.
func (s *Store) AddNodes(nodeID string, dependencies ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if nodeID == "" {
		return ErrEmptyNodeID
	}
	if err := s.ensureNode(nodeID); err != nil {
		return err
	}

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

		if err := s.ensureNode(dep); err != nil {
			return err
		}

		// If dep already reaches nodeID, adding nodeID->dep creates a cycle.
		if s.reaches(dep, nodeID) {
			return fmt.Errorf("circular dependency detected: %s -> %s would create a loop", nodeID, dep)
		}

		if _, ok := s.deps[nodeID][dep]; ok {
			continue // edge already exists
		}

		s.deps[nodeID][dep] = struct{}{}
		s.dependents[dep][nodeID] = struct{}{}
		s.byNode[nodeID].appendUnique(dep)
	}

	return nil
}

// RemoveNode removes nodeID if no other node depends on it.
// Also prunes orphan dependency branches that become unreferenced.
func (s *Store) RemoveNode(nodeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if nodeID == "" {
		return ErrEmptyNodeID
	}
	if _, ok := s.byNode[nodeID]; !ok {
		return ErrNodeNotFound
	}
	if len(s.dependents[nodeID]) > 0 {
		return ErrNodeIsDependency
	}

	children := keysSet(s.deps[nodeID])

	// Remove outgoing edges nodeID -> child.
	for child := range s.deps[nodeID] {
		delete(s.dependents[child], nodeID)
	}

	delete(s.deps, nodeID)
	delete(s.dependents, nodeID)
	s.removeList(nodeID)

	// Remove orphan sub-branches.
	for _, child := range children {
		s.pruneOrphan(child)
	}
	return nil
}

func (s *Store) ensureNode(nodeID string) error {
	if nodeID == "" {
		return ErrEmptyNodeID
	}
	if _, ok := s.byNode[nodeID]; ok {
		if _, ok := s.deps[nodeID]; !ok {
			s.deps[nodeID] = make(map[string]struct{})
		}
		if _, ok := s.dependents[nodeID]; !ok {
			s.dependents[nodeID] = make(map[string]struct{})
		}
		return nil
	}

	ll := newLinkedList(nodeID)
	s.byNode[nodeID] = ll
	s.index[nodeID] = len(s.allLists)
	s.allLists = append(s.allLists, ll)
	s.deps[nodeID] = make(map[string]struct{})
	s.dependents[nodeID] = make(map[string]struct{})
	return nil
}

// reaches checks if start depends (directly/indirectly) on target.
func (s *Store) reaches(start, target string) bool {
	if start == target {
		return true
	}
	visited := map[string]struct{}{}
	stack := []string{start}

	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if _, ok := visited[n]; ok {
			continue
		}
		visited[n] = struct{}{}

		for dep := range s.deps[n] {
			if dep == target {
				return true
			}
			stack = append(stack, dep)
		}
	}
	return false
}

func (s *Store) pruneOrphan(nodeID string) {
	if _, ok := s.byNode[nodeID]; !ok {
		return
	}
	if len(s.dependents[nodeID]) > 0 {
		return
	}

	deps := s.deps[nodeID]
	children := keysSet(deps)

	for child := range s.deps[nodeID] {
		delete(s.dependents[child], nodeID)
	}
	delete(s.deps, nodeID)
	delete(s.dependents, nodeID)
	s.removeList(nodeID)

	for _, child := range children {
		s.pruneOrphan(child)
	}
}

func (s *Store) removeList(nodeID string) {
	i, ok := s.index[nodeID]
	if !ok {
		return
	}
	last := len(s.allLists) - 1
	if i != last {
		s.allLists[i] = s.allLists[last]
		owner := s.allLists[i].head.id
		s.index[owner] = i
	}
	s.allLists[last] = nil
	s.allLists = s.allLists[:last]
	delete(s.index, nodeID)
	delete(s.byNode, nodeID)
}

func keysSet(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

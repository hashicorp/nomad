package scheduler

import (
	"math/rand"

	"github.com/hashicorp/nomad/nomad/structs"
)

// FeasibleIterator is used to iteratively yield nodes that
// match feasibility constraints. The iterators may manage
// some state for performance optimizations.
type FeasibleIterator interface {
	// Next yields a feasible node or nil if exhausted
	Next() *structs.Node
}

// StaticIterator is a FeasibleIterator which returns nodes
// in a static order. This is used at the base of the iterator
// chain only for testing due to deterministic behavior.
type StaticIterator struct {
	ctx    Context
	nodes  []*structs.Node
	offset int
}

// NewStaticIterator constructs a random iterator from a list of nodes
func NewStaticIterator(ctx Context, nodes []*structs.Node) *StaticIterator {
	iter := &StaticIterator{
		ctx:   ctx,
		nodes: nodes,
	}
	return iter
}

func (iter *StaticIterator) Next() *structs.Node {
	// Check if exhausted
	if iter.offset == len(iter.nodes) {
		return nil
	}

	// Return the next offset
	offset := iter.offset
	iter.offset += 1
	return iter.nodes[offset]
}

// NewRandomIterator constructs a static iterator from a list of nodes
// after applying the Fisher-Yates algorithm for a random shuffle. This
// is applied in-place
func NewRandomIterator(ctx Context, nodes []*structs.Node) *StaticIterator {
	// shuffle with the Fisher-Yates algorithm
	n := len(nodes)
	for i := n - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		nodes[i], nodes[j] = nodes[j], nodes[i]
	}

	// Create a static iterator
	return NewStaticIterator(ctx, nodes)
}

// ConstraintIterator is a FeasibleIterator which returns nodes
// that match a given set of constraints. This is used to filter
// on job, task group, and task constraints.
type ConstraintIterator struct {
	ctx         Context
	source      FeasibleIterator
	constraints []*structs.Constraint
}

// NewConstraintIterator creates a ConstraintIterator from a source and set of constraints
func NewConstraintIterator(ctx Context, source FeasibleIterator, constraints []*structs.Constraint) *ConstraintIterator {
	iter := &ConstraintIterator{
		ctx:         ctx,
		source:      source,
		constraints: constraints,
	}
	return iter
}

func (iter *ConstraintIterator) Next() *structs.Node {
	for {
		// Get the next option from the source
		option := iter.source.Next()
		if option == nil {
			return nil
		}

		// Use this node if possible
		if iter.meetsConstraints(option) {
			return option
		}
	}
}

func (iter *ConstraintIterator) meetsConstraints(option *structs.Node) bool {
	// TODO:
	return true
}

// DriverIterator is a FeasibleIterator which returns nodes that
// have the drivers necessary to scheduler a task group.
type DriverIterator struct {
	ctx     Context
	source  FeasibleIterator
	drivers map[string]struct{}
}

// NewDriverIterator creates a DriverIterator from a source and set of drivers
func NewDriverIterator(ctx Context, source FeasibleIterator, drivers map[string]struct{}) *DriverIterator {
	iter := &DriverIterator{
		ctx:     ctx,
		source:  source,
		drivers: drivers,
	}
	return iter
}

func (iter *DriverIterator) Next() *structs.Node {
	for {
		// Get the next option from the source
		option := iter.source.Next()
		if option == nil {
			return nil
		}

		// Use this node if possible
		if iter.hasDrivers(option) {
			return option
		}
	}
}

func (iter *DriverIterator) hasDrivers(option *structs.Node) bool {
	return true
}

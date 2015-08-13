package scheduler

import (
	"fmt"
	"math/rand"
	"reflect"
	"strings"

	"github.com/hashicorp/nomad/nomad/structs"
)

// FeasibleIterator is used to iteratively yield nodes that
// match feasibility constraints. The iterators may manage
// some state for performance optimizations.
type FeasibleIterator interface {
	// Next yields a feasible node or nil if exhausted
	Next() *structs.Node

	// Reset is invoked when an allocation has been placed
	// to reset any stale state.
	Reset()
}

// StaticIterator is a FeasibleIterator which returns nodes
// in a static order. This is used at the base of the iterator
// chain only for testing due to deterministic behavior.
type StaticIterator struct {
	ctx    Context
	nodes  []*structs.Node
	offset int
	seen   int
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
	n := len(iter.nodes)
	if iter.offset == n || iter.seen == n {
		if iter.seen != n {
			iter.offset = 0
		} else {
			return nil
		}
	}

	// Return the next offset
	offset := iter.offset
	iter.offset += 1
	iter.seen += 1
	return iter.nodes[offset]
}

func (iter *StaticIterator) Reset() {
	iter.seen = 0
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

func (iter *DriverIterator) SetDrivers(d map[string]struct{}) {
	iter.drivers = d
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

func (iter *DriverIterator) Reset() {
	iter.source.Reset()
}

// hasDrivers is used to check if the node has all the appropriate
// drivers for this task group. Drivers are registered as node attribute
// like "driver.docker=1" with their corresponding version.
func (iter *DriverIterator) hasDrivers(option *structs.Node) bool {
	for driver := range iter.drivers {
		driverStr := fmt.Sprintf("driver.%s", driver)
		_, ok := option.Attributes[driverStr]
		if !ok {
			return false
		}
	}
	return true
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

func (iter *ConstraintIterator) SetConstraints(c []*structs.Constraint) {
	iter.constraints = c
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

func (iter *ConstraintIterator) Reset() {
	iter.source.Reset()
}

func (iter *ConstraintIterator) meetsConstraints(option *structs.Node) bool {
	for _, constraint := range iter.constraints {
		if !iter.meetsConstraint(constraint, option) {
			return false
		}
	}
	return true
}

func (iter *ConstraintIterator) meetsConstraint(constraint *structs.Constraint, option *structs.Node) bool {
	// Only enforce hard constraints, soft constraints are used for ranking
	if !constraint.Hard {
		return true
	}

	// Resolve the targets
	lVal, ok := resolveConstraintTarget(constraint.LTarget, option)
	if !ok {
		return false
	}
	rVal, ok := resolveConstraintTarget(constraint.RTarget, option)
	if !ok {
		return false
	}

	// Check if satisfied
	return checkConstraint(constraint.Operand, lVal, rVal)
}

// resolveConstraintTarget is used to resolve the LTarget and RTarget of a Constraint
func resolveConstraintTarget(target string, node *structs.Node) (interface{}, bool) {
	// If no prefix, this must be a literal value
	if !strings.HasPrefix(target, "$") {
		return target, true
	}

	// Handle the interpolations
	switch {
	case "$node.id" == target:
		return node.ID, true

	case "$node.datacenter" == target:
		return node.Datacenter, true

	case "$node.name" == target:
		return node.Name, true

	case strings.HasPrefix(target, "$attr."):
		attr := strings.TrimPrefix(target, "$attr.")
		val, ok := node.Attributes[attr]
		return val, ok

	case strings.HasPrefix(target, "$meta."):
		meta := strings.TrimPrefix(target, "$meta.")
		val, ok := node.Meta[meta]
		return val, ok

	default:
		return nil, false
	}
}

// checkConstraint checks if a constraint is satisfied
func checkConstraint(operand string, lVal, rVal interface{}) bool {
	switch operand {
	case "=", "==", "is":
		return reflect.DeepEqual(lVal, rVal)
	case "!=", "not":
		return !reflect.DeepEqual(lVal, rVal)
	case "<", "<=", ">", ">=":
		// TODO: Implement
		return false
	case "contains":
		// TODO: Implement
		return false
	default:
		return false
	}
}

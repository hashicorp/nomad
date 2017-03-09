package scheduler

import (
	"fmt"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/structs"
)

// propertySet is used to track the values used for a particular property.
type propertySet struct {
	// ctx is used to lookup the plan and state
	ctx Context

	jobID string

	// constraint is the constraint this property set is checking
	constraint *structs.Constraint

	// errorBuilding marks whether there was an error when building the property
	// set
	errorBuilding error

	// existingValues is the set of values for the given property that have been
	// used by pre-existing allocations.
	existingValues map[string]struct{}

	// proposedValues is the set of values for the given property that are used
	// from proposed allocations.
	proposedValues map[string]struct{}

	// clearedValues is the set of values that are no longer being used by
	// existingValues because of proposed stops.
	clearedValues map[string]struct{}
}

// NewPropertySet returns a new property set used to guarantee unique property
// values for new allocation placements.
func NewPropertySet(ctx Context, job *structs.Job) *propertySet {
	p := &propertySet{
		ctx:            ctx,
		jobID:          job.ID,
		existingValues: make(map[string]struct{}),
	}

	return p
}

// SetJobConstraint is used to parameterize the property set for a
// distinct_property constraint set at the job level.
func (p *propertySet) SetJobConstraint(constraint *structs.Constraint) {
	// Store the constraint
	p.constraint = constraint

	// Retrieve all previously placed allocations
	ws := memdb.NewWatchSet()
	allocs, err := p.ctx.State().AllocsByJob(ws, p.jobID, false)
	if err != nil {
		p.errorBuilding = fmt.Errorf("failed to get job's allocations: %v", err)
		p.ctx.Logger().Printf("[ERR] scheduler.dynamic-constraint: %v", p.errorBuilding)
		return
	}

	// Only want running allocs
	filtered, _ := structs.FilterTerminalAllocs(allocs)

	// Get all the nodes that have been used by the allocs
	nodes, err := p.buildNodeMap(filtered)
	if err != nil {
		p.errorBuilding = err
		p.ctx.Logger().Printf("[ERR] scheduler.dynamic-constraint: %v", err)
		return
	}

	p.populateExisting(constraint, filtered, nodes)
}

// SetTGConstraint is used to parameterize the property set for a
// distinct_property constraint set at the task group level. The inputs are the
// constraint and the task group name.
func (p *propertySet) SetTGConstraint(constraint *structs.Constraint, taskGroup string) {
	// Store the constraint
	p.constraint = constraint

	// Retrieve all previously placed allocations
	ws := memdb.NewWatchSet()
	allocs, err := p.ctx.State().AllocsByJob(ws, p.jobID, false)
	if err != nil {
		p.errorBuilding = fmt.Errorf("failed to get job's allocations: %v", err)
		p.ctx.Logger().Printf("[ERR] scheduler.dynamic-constraint: %v", p.errorBuilding)
		return
	}

	// Only want running allocs from the task group
	n := len(allocs)
	for i := 0; i < n; i++ {
		if allocs[i].TaskGroup != taskGroup || allocs[i].TerminalStatus() {
			allocs[i], allocs[n-1] = allocs[n-1], nil
			i--
			n--
		}
	}
	allocs = allocs[:n]

	// Get all the nodes that have been used by the allocs
	nodes, err := p.buildNodeMap(allocs)
	if err != nil {
		p.errorBuilding = err
		p.ctx.Logger().Printf("[ERR] scheduler.dynamic-constraint: %v", err)
		return
	}

	p.populateExisting(constraint, allocs, nodes)
}

// populateExisting is a helper shared when setting the constraint to populate
// the existing values. The allocations should be filtered appropriately prior
// to calling.
func (p *propertySet) populateExisting(constraint *structs.Constraint, jobAllocs []*structs.Allocation, nodes map[string]*structs.Node) {
	for _, alloc := range jobAllocs {
		nProperty, ok := p.getProperty(nodes[alloc.NodeID], constraint.LTarget)
		if !ok {
			continue
		}

		p.existingValues[nProperty] = struct{}{}
	}
}

// PopulateProposed populates the proposed values and recomputes any cleared
// value. It should be called whenever the plan is updated to ensure correct
// results when checking an option.
func (p *propertySet) PopulateProposed() {

	// Reset the proposed properties
	p.proposedValues = make(map[string]struct{})
	p.clearedValues = make(map[string]struct{})

	// Gather the set of proposed stops.
	var stopping []*structs.Allocation
	for _, updates := range p.ctx.Plan().NodeUpdate {
		stopping = append(stopping, updates...)
	}

	// Gather the proposed allocations
	var proposed []*structs.Allocation
	for _, pallocs := range p.ctx.Plan().NodeAllocation {
		proposed = append(proposed, pallocs...)
	}
	proposed, _ = structs.FilterTerminalAllocs(proposed)

	// Get the used nodes
	both := make([]*structs.Allocation, 0, len(stopping)+len(proposed))
	both = append(both, stopping...)
	both = append(both, proposed...)
	nodes, err := p.buildNodeMap(both)
	if err != nil {
		p.errorBuilding = err
		p.ctx.Logger().Printf("[ERR] scheduler.dynamic-constraint: %v", err)
		return
	}

	// Populate the cleared values
	for _, alloc := range stopping {
		nProperty, ok := p.getProperty(nodes[alloc.NodeID], p.constraint.LTarget)
		if !ok {
			continue
		}

		p.clearedValues[nProperty] = struct{}{}
	}

	// Populate the proposed values
	for _, alloc := range proposed {
		nProperty, ok := p.getProperty(nodes[alloc.NodeID], p.constraint.LTarget)
		if !ok {
			continue
		}

		p.proposedValues[nProperty] = struct{}{}

		// If it was cleared, it is now being used
		delete(p.clearedValues, nProperty)
	}
}

// SatisfiesDistinctProperties checks if the option satisfies the
// distinct_property constraints given the existing placements and proposed
// placements. If the option does not satisfy the constraints an explanation is
// given.
func (p *propertySet) SatisfiesDistinctProperties(option *structs.Node, tg string) (bool, string) {
	// Check if there was an error building
	if p.errorBuilding != nil {
		return false, p.errorBuilding.Error()
	}

	// Get the nodes property value
	nValue, ok := p.getProperty(option, p.constraint.LTarget)
	if !ok {
		return false, fmt.Sprintf("missing property %q", p.constraint.LTarget)
	}

	// both is used to iterate over both the proposed and existing used
	// properties
	bothAll := []map[string]struct{}{p.existingValues, p.proposedValues}

	// Check if the nodes value has already been used.
	for _, usedProperties := range bothAll {
		// Check if the nodes value has been used
		_, used := usedProperties[nValue]
		if !used {
			continue
		}

		// Check if the value has been cleared from a proposed stop
		if _, cleared := p.clearedValues[nValue]; cleared {
			continue
		}

		return false, fmt.Sprintf("distinct_property: %s=%s already used", p.constraint.LTarget, nValue)
	}

	return true, ""
}

// buildNodeMap takes a list of allocations and returns a map of the nodes used
// by those allocations
func (p *propertySet) buildNodeMap(allocs []*structs.Allocation) (map[string]*structs.Node, error) {
	// Get all the nodes that have been used by the allocs
	nodes := make(map[string]*structs.Node)
	ws := memdb.NewWatchSet()
	for _, alloc := range allocs {
		if _, ok := nodes[alloc.NodeID]; ok {
			continue
		}

		node, err := p.ctx.State().NodeByID(ws, alloc.NodeID)
		if err != nil {
			return nil, fmt.Errorf("failed to lookup node ID %q: %v", alloc.NodeID, err)
		}

		nodes[alloc.NodeID] = node
	}

	return nodes, nil
}

// getProperty is used to lookup the property value on the node
func (p *propertySet) getProperty(n *structs.Node, property string) (string, bool) {
	if n == nil || property == "" {
		return "", false
	}

	val, ok := resolveConstraintTarget(property, n)
	if !ok {
		return "", false
	}
	nodeValue, ok := val.(string)
	if !ok {
		return "", false
	}

	return nodeValue, true
}

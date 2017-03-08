package scheduler

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-version"
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

// FeasibilityChecker is used to check if a single node meets feasibility
// constraints.
type FeasibilityChecker interface {
	Feasible(*structs.Node) bool
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
	iter.ctx.Metrics().EvaluateNode()
	return iter.nodes[offset]
}

func (iter *StaticIterator) Reset() {
	iter.seen = 0
}

func (iter *StaticIterator) SetNodes(nodes []*structs.Node) {
	iter.nodes = nodes
	iter.offset = 0
	iter.seen = 0
}

// NewRandomIterator constructs a static iterator from a list of nodes
// after applying the Fisher-Yates algorithm for a random shuffle. This
// is applied in-place
func NewRandomIterator(ctx Context, nodes []*structs.Node) *StaticIterator {
	// shuffle with the Fisher-Yates algorithm
	shuffleNodes(nodes)

	// Create a static iterator
	return NewStaticIterator(ctx, nodes)
}

// DriverChecker is a FeasibilityChecker which returns whether a node has the
// drivers necessary to scheduler a task group.
type DriverChecker struct {
	ctx     Context
	drivers map[string]struct{}
}

// NewDriverChecker creates a DriverChecker from a set of drivers
func NewDriverChecker(ctx Context, drivers map[string]struct{}) *DriverChecker {
	return &DriverChecker{
		ctx:     ctx,
		drivers: drivers,
	}
}

func (c *DriverChecker) SetDrivers(d map[string]struct{}) {
	c.drivers = d
}

func (c *DriverChecker) Feasible(option *structs.Node) bool {
	// Use this node if possible
	if c.hasDrivers(option) {
		return true
	}
	c.ctx.Metrics().FilterNode(option, "missing drivers")
	return false
}

// hasDrivers is used to check if the node has all the appropriate
// drivers for this task group. Drivers are registered as node attribute
// like "driver.docker=1" with their corresponding version.
func (c *DriverChecker) hasDrivers(option *structs.Node) bool {
	for driver := range c.drivers {
		driverStr := fmt.Sprintf("driver.%s", driver)
		value, ok := option.Attributes[driverStr]
		if !ok {
			return false
		}

		enabled, err := strconv.ParseBool(value)
		if err != nil {
			c.ctx.Logger().
				Printf("[WARN] scheduler.DriverChecker: node %v has invalid driver setting %v: %v",
					option.ID, driverStr, value)
			return false
		}

		if !enabled {
			return false
		}
	}
	return true
}

// ProposedAllocConstraintIterator is a FeasibleIterator which returns nodes that
// match constraints that are not static such as Node attributes but are
// effected by proposed alloc placements. Examples are distinct_hosts and
// tenancy constraints. This is used to filter on job and task group
// constraints.
type ProposedAllocConstraintIterator struct {
	ctx    Context
	source FeasibleIterator
	tg     *structs.TaskGroup
	job    *structs.Job

	// distinctProperties is used to track the distinct properties of the job
	// and to check if node options satisfy these constraints.
	distinctProperties *propertySet

	// Store whether the Job or TaskGroup has a distinct_hosts constraints so
	// they don't have to be calculated every time Next() is called.
	tgDistinctHosts  bool
	jobDistinctHosts bool
}

// NewProposedAllocConstraintIterator creates a ProposedAllocConstraintIterator
// from a source.
func NewProposedAllocConstraintIterator(ctx Context, source FeasibleIterator) *ProposedAllocConstraintIterator {
	return &ProposedAllocConstraintIterator{
		ctx:                ctx,
		source:             source,
		distinctProperties: NewPropertySet(ctx),
	}
}

func (iter *ProposedAllocConstraintIterator) SetTaskGroup(tg *structs.TaskGroup) {
	iter.tg = tg
	iter.tgDistinctHosts = iter.hasDistinctHostsConstraint(tg.Constraints)
}

func (iter *ProposedAllocConstraintIterator) SetJob(job *structs.Job) {
	iter.job = job
	iter.jobDistinctHosts = iter.hasDistinctHostsConstraint(job.Constraints)

	if err := iter.distinctProperties.SetJob(job); err != nil {
		iter.ctx.Logger().Printf(
			"[ERR] scheduler.dynamic-constraint: failed to build property set: %v", err)
	}
}

func (iter *ProposedAllocConstraintIterator) hasDistinctHostsConstraint(constraints []*structs.Constraint) bool {
	for _, con := range constraints {
		if con.Operand == structs.ConstraintDistinctHosts {
			return true
		}
	}

	return false
}

func (iter *ProposedAllocConstraintIterator) Next() *structs.Node {
	for {
		// Get the next option from the source
		option := iter.source.Next()

		// Hot-path if the option is nil or there are no distinct_hosts or
		// distinct_property constraints.
		hosts := iter.jobDistinctHosts || iter.tgDistinctHosts
		properties := iter.distinctProperties.HasDistinctPropertyConstraints()
		if option == nil || !(hosts || properties) {
			return option
		}

		// Check if the host constraints are satisfied
		if hosts {
			if !iter.satisfiesDistinctHosts(option) {
				iter.ctx.Metrics().FilterNode(option, structs.ConstraintDistinctHosts)
				continue
			}
		}

		// Check if the property constraints are satisfied
		if properties {
			satisfied, reason := iter.distinctProperties.SatisfiesDistinctProperties(option, iter.tg.Name)
			if !satisfied {
				iter.ctx.Metrics().FilterNode(option, reason)
				continue
			}
		}

		return option
	}
}

// satisfiesDistinctHosts checks if the node satisfies a distinct_hosts
// constraint either specified at the job level or the TaskGroup level.
func (iter *ProposedAllocConstraintIterator) satisfiesDistinctHosts(option *structs.Node) bool {
	// Check if there is no constraint set.
	if !(iter.jobDistinctHosts || iter.tgDistinctHosts) {
		return true
	}

	// Get the proposed allocations
	proposed, err := iter.ctx.ProposedAllocs(option.ID)
	if err != nil {
		iter.ctx.Logger().Printf(
			"[ERR] scheduler.dynamic-constraint: failed to get proposed allocations: %v", err)
		return false
	}

	// Skip the node if the task group has already been allocated on it.
	for _, alloc := range proposed {
		// If the job has a distinct_hosts constraint we only need an alloc
		// collision on the JobID but if the constraint is on the TaskGroup then
		// we need both a job and TaskGroup collision.
		jobCollision := alloc.JobID == iter.job.ID
		taskCollision := alloc.TaskGroup == iter.tg.Name
		if iter.jobDistinctHosts && jobCollision || jobCollision && taskCollision {
			return false
		}
	}

	return true
}

func (iter *ProposedAllocConstraintIterator) Reset() {
	iter.source.Reset()

	// Repopulate the proposed set every time we are reset because an
	// additional allocation may have been added
	if err := iter.distinctProperties.PopulateProposed(); err != nil {
		iter.ctx.Logger().Printf(
			"[ERR] scheduler.dynamic-constraint: failed to populate proposed properties: %v", err)
	}
}

// propertySet is used to track used values for a particular node property.
type propertySet struct {

	// ctx is used to lookup the plan and state
	ctx Context

	// job stores the job the property set is tracking
	job *structs.Job

	// jobConstrainedProperties stores the set of LTargets that we are
	// constrained on. The outer key is the task group name. The values stored
	// in key "" are those constrained at the job level.
	jobConstrainedProperties map[string]map[string]struct{}

	// existingProperties is a mapping of task group/job to properties (LTarget)
	// to a string set of used values for pre-placed allocation.
	existingProperties map[string]map[string]map[string]struct{}

	// proposedCreateProperties is a mapping of task group/job to properties (LTarget)
	// to a string set of used values for proposed allocations.
	proposedCreateProperties map[string]map[string]map[string]struct{}

	// clearedProposedProperties is a mapping of task group/job to properties (LTarget)
	// to a string set of values that have been cleared and are no longer used
	clearedProposedProperties map[string]map[string]map[string]struct{}
}

// NewPropertySet returns a new property set used to guarantee unique property
// values for new allocation placements.
func NewPropertySet(ctx Context) *propertySet {
	p := &propertySet{
		ctx: ctx,
		jobConstrainedProperties:  make(map[string]map[string]struct{}),
		existingProperties:        make(map[string]map[string]map[string]struct{}),
		proposedCreateProperties:  make(map[string]map[string]map[string]struct{}),
		clearedProposedProperties: make(map[string]map[string]map[string]struct{}),
	}

	return p
}

// setJob sets the job the property set is tracking. The distinct property
// constraints and property values already used by existing allocations are
// calculated.
func (p *propertySet) SetJob(j *structs.Job) error {
	p.job = j

	p.buildDistinctProperties()
	if err := p.populateExisting(); err != nil {
		return err
	}

	return nil
}

// HasDistinctPropertyConstraints returns whether there are distinct_property
// constraints on the job.
func (p *propertySet) HasDistinctPropertyConstraints() bool {
	return len(p.jobConstrainedProperties) != 0
}

// PopulateProposed populates the set of property values used by the proposed
// allocations for the job. This should be called on every reset
func (p *propertySet) PopulateProposed() error {
	// Hot path since there is nothing to do
	if !p.HasDistinctPropertyConstraints() {
		return nil
	}

	// Reset the proposed properties
	p.proposedCreateProperties = make(map[string]map[string]map[string]struct{})
	p.clearedProposedProperties = make(map[string]map[string]map[string]struct{})

	// Gather the set of proposed stops.
	var stopping []*structs.Allocation
	for _, updates := range p.ctx.Plan().NodeUpdate {
		stopping = append(stopping, updates...)
	}

	// build the property set for the proposed stopped allocations
	// This should be called before building the property set for the created
	// allocs.
	if err := p.buildProperySet(stopping, false, true); err != nil {
		return err
	}

	// Gather the proposed allocations
	var proposed []*structs.Allocation
	for _, pallocs := range p.ctx.Plan().NodeAllocation {
		proposed = append(proposed, pallocs...)
	}

	// build the property set for the proposed new allocations
	return p.buildProperySet(proposed, false, false)
}

// satisfiesDistinctProperties checks if the option satisfies all
// distinct_property constraints given the existing placements and proposed
// placements. If the option does not satisfy the constraints an explanation is
// given.
func (p *propertySet) SatisfiesDistinctProperties(option *structs.Node, tg string) (bool, string) {
	// Hot path if there is nothing to do
	jobConstrainedProperties := p.jobConstrainedProperties[""]
	tgConstrainedProperties := p.jobConstrainedProperties[tg]
	if len(jobConstrainedProperties) == 0 && len(tgConstrainedProperties) == 0 {
		return true, ""
	}

	// both is used to iterate over both the proposed and existing used
	// properties
	bothAll := []map[string]map[string]map[string]struct{}{p.existingProperties, p.proposedCreateProperties}

	// Check if the option is unique for all the job properties
	for constrainedProperty := range jobConstrainedProperties {
		// Get the nodes property value
		nValue, ok := p.getProperty(option, constrainedProperty)
		if !ok {
			return false, fmt.Sprintf("missing property %q", constrainedProperty)
		}

		// Check if the nodes value has already been used.
		for _, usedProperties := range bothAll {
			// Since we are checking at the job level, check all task groups
			for group, properties := range usedProperties {
				setValues, ok := properties[constrainedProperty]
				if !ok {
					continue
				}

				// Check if the nodes value has been used
				_, used := setValues[nValue]
				if !used {
					continue
				}

				// The last check is to ensure that the value hasn't been
				// removed in the proposed removals.
				if _, cleared := p.clearedProposedProperties[group][constrainedProperty][nValue]; cleared {
					continue
				}

				return false, fmt.Sprintf("distinct_property: %s=%s already used", constrainedProperty, nValue)
			}
		}
	}

	// bothTG is both filtered at by the task group
	bothTG := []map[string]map[string]struct{}{p.existingProperties[tg], p.proposedCreateProperties[tg]}

	// Check if the option is unique for all the task group properties
	for constrainedProperty := range tgConstrainedProperties {
		// Get the nodes property value
		nValue, ok := p.getProperty(option, constrainedProperty)
		if !ok {
			return false, fmt.Sprintf("missing property %q", constrainedProperty)
		}

		// Check if the nodes value has already been used.
		for _, properties := range bothTG {
			setValues, ok := properties[constrainedProperty]
			if !ok {
				continue
			}

			// Check if the nodes value has been used
			if _, used := setValues[nValue]; !used {
				continue
			}

			// The last check is to ensure that the value hasn't been
			// removed in the proposed removals.
			if _, cleared := p.clearedProposedProperties[tg][constrainedProperty][nValue]; cleared {
				continue
			}

			return false, fmt.Sprintf("distinct_property: %s=%s already used", constrainedProperty, nValue)
		}
	}

	return true, ""
}

// buildDistinctProperties takes the job and populates the map of distinct
// properties that are constrained on for the job.
func (p *propertySet) buildDistinctProperties() {
	for _, c := range p.job.Constraints {
		if c.Operand != structs.ConstraintDistinctProperty {
			continue
		}

		// Store job properties in the magic empty string since it can't be used
		// by any task group.
		if _, ok := p.jobConstrainedProperties[""]; !ok {
			p.jobConstrainedProperties[""] = make(map[string]struct{})
		}

		p.jobConstrainedProperties[""][c.LTarget] = struct{}{}
	}

	for _, tg := range p.job.TaskGroups {
		for _, c := range tg.Constraints {
			if c.Operand != structs.ConstraintDistinctProperty {
				continue
			}

			if _, ok := p.jobConstrainedProperties[tg.Name]; !ok {
				p.jobConstrainedProperties[tg.Name] = make(map[string]struct{})
			}

			p.jobConstrainedProperties[tg.Name][c.LTarget] = struct{}{}
		}
	}
}

// populateExisting populates the set of property values used by existing
// allocations for the job.
func (p *propertySet) populateExisting() error {
	// Hot path since there is nothing to do
	if !p.HasDistinctPropertyConstraints() {
		return nil
	}

	// Retrieve all previously placed allocations
	ws := memdb.NewWatchSet()
	allocs, err := p.ctx.State().AllocsByJob(ws, p.job.ID, false)
	if err != nil {
		return fmt.Errorf("failed to get job's allocations: %v", err)
	}

	return p.buildProperySet(allocs, true, false)
}

// buildProperySet takes a set of allocations and determines what property
// values have been used by them. The existing boolean marks whether these are
// existing allocations or proposed allocations. Stopping marks whether the
// allocations are being stopped.
func (p *propertySet) buildProperySet(allocs []*structs.Allocation, existing, stopping bool) error {
	// Only want running allocs
	filtered := allocs
	if !stopping {
		filtered, _ = structs.FilterTerminalAllocs(allocs)
	}

	// Get all the nodes that have been used by the allocs
	ws := memdb.NewWatchSet()
	nodes := make(map[string]*structs.Node)
	for _, alloc := range filtered {
		if _, ok := nodes[alloc.NodeID]; ok {
			continue
		}

		node, err := p.ctx.State().NodeByID(ws, alloc.NodeID)
		if err != nil {
			return fmt.Errorf("failed to lookup node ID %q: %v", alloc.NodeID, err)
		}

		nodes[alloc.NodeID] = node
	}

	// propertySet is the set we are operating on
	propertySet := p.existingProperties
	if !existing && !stopping {
		propertySet = p.proposedCreateProperties
	} else if stopping {
		propertySet = p.clearedProposedProperties
	}

	// Go through each allocation and build the set of property values that have
	// been used
	for _, alloc := range filtered {
		// Gather job related constrained properties
		jobProperties := p.jobConstrainedProperties[""]
		for constrainedProperty := range jobProperties {
			nProperty, ok := p.getProperty(nodes[alloc.NodeID], constrainedProperty)
			if !ok {
				continue
			}

			if _, exists := propertySet[""]; !exists {
				propertySet[""] = make(map[string]map[string]struct{})
			}

			if _, exists := propertySet[""][constrainedProperty]; !exists {
				propertySet[""][constrainedProperty] = make(map[string]struct{})
			}

			propertySet[""][constrainedProperty][nProperty] = struct{}{}

			// This is a newly created allocation so clear out the fact that
			// proposed property is not being used anymore
			if !existing && !stopping {
				delete(p.clearedProposedProperties[""][constrainedProperty], nProperty)
			}
		}

		// Gather task group related constrained properties
		groupProperties := p.jobConstrainedProperties[alloc.TaskGroup]
		for constrainedProperty := range groupProperties {
			nProperty, ok := p.getProperty(nodes[alloc.NodeID], constrainedProperty)
			if !ok {
				continue
			}

			if _, exists := propertySet[alloc.TaskGroup]; !exists {
				propertySet[alloc.TaskGroup] = make(map[string]map[string]struct{})
			}
			if _, exists := propertySet[alloc.TaskGroup][constrainedProperty]; !exists {
				propertySet[alloc.TaskGroup][constrainedProperty] = make(map[string]struct{})
			}

			propertySet[alloc.TaskGroup][constrainedProperty][nProperty] = struct{}{}

			// This is a newly created allocation so clear out the fact that
			// proposed property is not being used anymore
			if !existing && !stopping {
				delete(p.clearedProposedProperties[alloc.TaskGroup][constrainedProperty], nProperty)
			}
		}
	}

	return nil
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

// ConstraintChecker is a FeasibilityChecker which returns nodes that match a
// given set of constraints. This is used to filter on job, task group, and task
// constraints.
type ConstraintChecker struct {
	ctx         Context
	constraints []*structs.Constraint
}

// NewConstraintChecker creates a ConstraintChecker for a set of constraints
func NewConstraintChecker(ctx Context, constraints []*structs.Constraint) *ConstraintChecker {
	return &ConstraintChecker{
		ctx:         ctx,
		constraints: constraints,
	}
}

func (c *ConstraintChecker) SetConstraints(constraints []*structs.Constraint) {
	c.constraints = constraints
}

func (c *ConstraintChecker) Feasible(option *structs.Node) bool {
	// Use this node if possible
	for _, constraint := range c.constraints {
		if !c.meetsConstraint(constraint, option) {
			c.ctx.Metrics().FilterNode(option, constraint.String())
			return false
		}
	}
	return true
}

func (c *ConstraintChecker) meetsConstraint(constraint *structs.Constraint, option *structs.Node) bool {
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
	return checkConstraint(c.ctx, constraint.Operand, lVal, rVal)
}

// resolveConstraintTarget is used to resolve the LTarget and RTarget of a Constraint
func resolveConstraintTarget(target string, node *structs.Node) (interface{}, bool) {
	// If no prefix, this must be a literal value
	if !strings.HasPrefix(target, "${") {
		return target, true
	}

	// Handle the interpolations
	switch {
	case "${node.unique.id}" == target:
		return node.ID, true

	case "${node.datacenter}" == target:
		return node.Datacenter, true

	case "${node.unique.name}" == target:
		return node.Name, true

	case "${node.class}" == target:
		return node.NodeClass, true

	case strings.HasPrefix(target, "${attr."):
		attr := strings.TrimSuffix(strings.TrimPrefix(target, "${attr."), "}")
		val, ok := node.Attributes[attr]
		return val, ok

	case strings.HasPrefix(target, "${meta."):
		meta := strings.TrimSuffix(strings.TrimPrefix(target, "${meta."), "}")
		val, ok := node.Meta[meta]
		return val, ok

	default:
		return nil, false
	}
}

// checkConstraint checks if a constraint is satisfied
func checkConstraint(ctx Context, operand string, lVal, rVal interface{}) bool {
	// Check for constraints not handled by this checker.
	switch operand {
	case structs.ConstraintDistinctHosts, structs.ConstraintDistinctProperty:
		return true
	default:
		break
	}

	switch operand {
	case "=", "==", "is":
		return reflect.DeepEqual(lVal, rVal)
	case "!=", "not":
		return !reflect.DeepEqual(lVal, rVal)
	case "<", "<=", ">", ">=":
		return checkLexicalOrder(operand, lVal, rVal)
	case structs.ConstraintVersion:
		return checkVersionConstraint(ctx, lVal, rVal)
	case structs.ConstraintRegex:
		return checkRegexpConstraint(ctx, lVal, rVal)
	case structs.ConstraintSetContains:
		return checkSetContainsConstraint(ctx, lVal, rVal)
	default:
		return false
	}
}

// checkLexicalOrder is used to check for lexical ordering
func checkLexicalOrder(op string, lVal, rVal interface{}) bool {
	// Ensure the values are strings
	lStr, ok := lVal.(string)
	if !ok {
		return false
	}
	rStr, ok := rVal.(string)
	if !ok {
		return false
	}

	switch op {
	case "<":
		return lStr < rStr
	case "<=":
		return lStr <= rStr
	case ">":
		return lStr > rStr
	case ">=":
		return lStr >= rStr
	default:
		return false
	}
}

// checkVersionConstraint is used to compare a version on the
// left hand side with a set of constraints on the right hand side
func checkVersionConstraint(ctx Context, lVal, rVal interface{}) bool {
	// Parse the version
	var versionStr string
	switch v := lVal.(type) {
	case string:
		versionStr = v
	case int:
		versionStr = fmt.Sprintf("%d", v)
	default:
		return false
	}

	// Parse the version
	vers, err := version.NewVersion(versionStr)
	if err != nil {
		return false
	}

	// Constraint must be a string
	constraintStr, ok := rVal.(string)
	if !ok {
		return false
	}

	// Check the cache for a match
	cache := ctx.ConstraintCache()
	constraints := cache[constraintStr]

	// Parse the constraints
	if constraints == nil {
		constraints, err = version.NewConstraint(constraintStr)
		if err != nil {
			return false
		}
		cache[constraintStr] = constraints
	}

	// Check the constraints against the version
	return constraints.Check(vers)
}

// checkRegexpConstraint is used to compare a value on the
// left hand side with a regexp on the right hand side
func checkRegexpConstraint(ctx Context, lVal, rVal interface{}) bool {
	// Ensure left-hand is string
	lStr, ok := lVal.(string)
	if !ok {
		return false
	}

	// Regexp must be a string
	regexpStr, ok := rVal.(string)
	if !ok {
		return false
	}

	// Check the cache
	cache := ctx.RegexpCache()
	re := cache[regexpStr]

	// Parse the regexp
	if re == nil {
		var err error
		re, err = regexp.Compile(regexpStr)
		if err != nil {
			return false
		}
		cache[regexpStr] = re
	}

	// Look for a match
	return re.MatchString(lStr)
}

// checkSetContainsConstraint is used to see if the left hand side contains the
// string on the right hand side
func checkSetContainsConstraint(ctx Context, lVal, rVal interface{}) bool {
	// Ensure left-hand is string
	lStr, ok := lVal.(string)
	if !ok {
		return false
	}

	// Regexp must be a string
	rStr, ok := rVal.(string)
	if !ok {
		return false
	}

	input := strings.Split(lStr, ",")
	lookup := make(map[string]struct{}, len(input))
	for _, in := range input {
		cleaned := strings.TrimSpace(in)
		lookup[cleaned] = struct{}{}
	}

	for _, r := range strings.Split(rStr, ",") {
		cleaned := strings.TrimSpace(r)
		if _, ok := lookup[cleaned]; !ok {
			return false
		}
	}

	return true
}

// FeasibilityWrapper is a FeasibleIterator which wraps both job and task group
// FeasibilityCheckers in which feasibility checking can be skipped if the
// computed node class has previously been marked as eligible or ineligible.
type FeasibilityWrapper struct {
	ctx         Context
	source      FeasibleIterator
	jobCheckers []FeasibilityChecker
	tgCheckers  []FeasibilityChecker
	tg          string
}

// NewFeasibilityWrapper returns a FeasibleIterator based on the passed source
// and FeasibilityCheckers.
func NewFeasibilityWrapper(ctx Context, source FeasibleIterator,
	jobCheckers, tgCheckers []FeasibilityChecker) *FeasibilityWrapper {
	return &FeasibilityWrapper{
		ctx:         ctx,
		source:      source,
		jobCheckers: jobCheckers,
		tgCheckers:  tgCheckers,
	}
}

func (w *FeasibilityWrapper) SetTaskGroup(tg string) {
	w.tg = tg
}

func (w *FeasibilityWrapper) Reset() {
	w.source.Reset()
}

// Next returns an eligible node, only running the FeasibilityCheckers as needed
// based on the sources computed node class.
func (w *FeasibilityWrapper) Next() *structs.Node {
	evalElig := w.ctx.Eligibility()
	metrics := w.ctx.Metrics()

OUTER:
	for {
		// Get the next option from the source
		option := w.source.Next()
		if option == nil {
			return nil
		}

		// Check if the job has been marked as eligible or ineligible.
		jobEscaped, jobUnknown := false, false
		switch evalElig.JobStatus(option.ComputedClass) {
		case EvalComputedClassIneligible:
			// Fast path the ineligible case
			metrics.FilterNode(option, "computed class ineligible")
			continue
		case EvalComputedClassEscaped:
			jobEscaped = true
		case EvalComputedClassUnknown:
			jobUnknown = true
		}

		// Run the job feasibility checks.
		for _, check := range w.jobCheckers {
			feasible := check.Feasible(option)
			if !feasible {
				// If the job hasn't escaped, set it to be ineligible since it
				// failed a job check.
				if !jobEscaped {
					evalElig.SetJobEligibility(false, option.ComputedClass)
				}
				continue OUTER
			}
		}

		// Set the job eligibility if the constraints weren't escaped and it
		// hasn't been set before.
		if !jobEscaped && jobUnknown {
			evalElig.SetJobEligibility(true, option.ComputedClass)
		}

		// Check if the task group has been marked as eligible or ineligible.
		tgEscaped, tgUnknown := false, false
		switch evalElig.TaskGroupStatus(w.tg, option.ComputedClass) {
		case EvalComputedClassIneligible:
			// Fast path the ineligible case
			metrics.FilterNode(option, "computed class ineligible")
			continue
		case EvalComputedClassEligible:
			// Fast path the eligible case
			return option
		case EvalComputedClassEscaped:
			tgEscaped = true
		case EvalComputedClassUnknown:
			tgUnknown = true
		}

		// Run the task group feasibility checks.
		for _, check := range w.tgCheckers {
			feasible := check.Feasible(option)
			if !feasible {
				// If the task group hasn't escaped, set it to be ineligible
				// since it failed a check.
				if !tgEscaped {
					evalElig.SetTaskGroupEligibility(false, w.tg, option.ComputedClass)
				}
				continue OUTER
			}
		}

		// Set the task group eligibility if the constraints weren't escaped and
		// it hasn't been set before.
		if !tgEscaped && tgUnknown {
			evalElig.SetTaskGroupEligibility(true, w.tg, option.ComputedClass)
		}

		return option
	}
}

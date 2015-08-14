package scheduler

import (
	"math"

	"github.com/hashicorp/nomad/nomad/structs"
)

// Stack is a chained collection of iterators
type Stack interface {
	// SetTaskGroup is used to set the job for selection
	SetJob(job *structs.Job)

	// SetTaskGroup is used to set the task group for selection.
	// This must be called in between calls to Select.
	SetTaskGroup(tg *structs.TaskGroup)

	// TaskGroupSize returns the size of the task group.
	// This is only valid after calling SetTaskGroup
	TaskGroupSize() *structs.Resources

	// Select is used to select a node for the task group
	Select() *RankedNode
}

// ServiceStack is used to hold pointers to each of the
// iterators which are chained together to do selection.
// Half of the stack is used for feasibility checking, while
// the second half of the stack is used for ranking and selection.
type ServiceStack struct {
	Context             Context
	BaseNodes           []*structs.Node
	Source              *StaticIterator
	JobConstraint       *ConstraintIterator
	TaskGroupDrivers    *DriverIterator
	TaskGroupConstraint *ConstraintIterator
	RankSource          *FeasibleRankIterator
	BinPack             *BinPackIterator
	Limit               *LimitIterator
	MaxScore            *MaxScoreIterator

	size *structs.Resources
}

// NewServiceStack constructs a stack used for selecting service placements
func NewServiceStack(ctx Context, baseNodes []*structs.Node) *ServiceStack {
	// Create a new stack
	stack := &ServiceStack{
		Context:   ctx,
		BaseNodes: baseNodes,
	}

	// Create the source iterator. We randomize the order we visit nodes
	// to reduce collisions between schedulers and to do a basic load
	// balancing across eligible nodes.
	stack.Source = NewRandomIterator(ctx, baseNodes)

	// Attach the job constraints. The job is filled in later.
	stack.JobConstraint = NewConstraintIterator(ctx, stack.Source, nil)

	// Filter on task group drivers first as they are faster
	stack.TaskGroupDrivers = NewDriverIterator(ctx, stack.JobConstraint, nil)

	// Filter on task group constraints second
	stack.TaskGroupConstraint = NewConstraintIterator(ctx, stack.TaskGroupDrivers, nil)

	// Upgrade from feasible to rank iterator
	stack.RankSource = NewFeasibleRankIterator(ctx, stack.TaskGroupConstraint)

	// Apply the bin packing, this depends on the resources needed by a particular task group.
	stack.BinPack = NewBinPackIterator(ctx, stack.RankSource, nil, true, 0)

	// Apply a limit function. This is to avoid scanning *every* possible node.
	// Instead we need to visit "enough". Using a log of the total number of
	// nodes is a good restriction, with at least 2 as the floor
	limit := 2
	if n := len(baseNodes); n > 0 {
		logLimit := int(math.Ceil(math.Log2(float64(n))))
		if logLimit > limit {
			limit = logLimit
		}
	}
	stack.Limit = NewLimitIterator(ctx, stack.BinPack, limit)

	// Select the node with the maximum score for placement
	stack.MaxScore = NewMaxScoreIterator(ctx, stack.Limit)
	return stack
}

func (s *ServiceStack) SetJob(job *structs.Job) {
	s.JobConstraint.SetConstraints(job.Constraints)
	s.BinPack.SetPriority(job.Priority)
}

func (s *ServiceStack) SetTaskGroup(tg *structs.TaskGroup) {
	// Collect the constraints, drivers and resources required by each
	// sub-task to aggregate the TaskGroup totals
	constr := make([]*structs.Constraint, 0, len(tg.Constraints))
	drivers := make(map[string]struct{})
	size := new(structs.Resources)
	constr = append(constr, tg.Constraints...)
	for _, task := range tg.Tasks {
		drivers[task.Driver] = struct{}{}
		constr = append(constr, task.Constraints...)
		size.Add(task.Resources)
	}

	// Store the size
	s.size = size

	// Update the parameters of iterators
	s.TaskGroupDrivers.SetDrivers(drivers)
	s.TaskGroupConstraint.SetConstraints(constr)
	s.BinPack.SetResources(size)

	// Reset the max selector
	s.MaxScore.Reset()
}

func (s *ServiceStack) TaskGroupSize() *structs.Resources {
	return s.size
}

func (s *ServiceStack) Select() *RankedNode {
	return s.MaxScore.Next()
}

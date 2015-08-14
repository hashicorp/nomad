package scheduler

import (
	"math"

	"github.com/hashicorp/nomad/nomad/structs"
)

// Stack is a chained collection of iterators. The stack is used to
// make placement decisions. Different schedulers may customize the
// stack they use to vary the way placements are made.
type Stack interface {
	// SetTaskGroup is used to set the job for selection
	SetJob(job *structs.Job)

	// Select is used to select a node for the task group
	Select(tg *structs.TaskGroup) (*RankedNode, *structs.Resources)
}

// ServiceStack is the Stack used for the Service scheduler. It is
// designed to make better placement decisions at the cost of performance.
type ServiceStack struct {
	ctx                 Context
	jobConstraint       *ConstraintIterator
	taskGroupDrivers    *DriverIterator
	taskGroupConstraint *ConstraintIterator
	binPack             *BinPackIterator
	maxScore            *MaxScoreIterator
}

// NewServiceStack constructs a stack used for selecting service placements
func NewServiceStack(ctx Context, baseNodes []*structs.Node) *ServiceStack {
	// Create a new stack
	stack := &ServiceStack{
		ctx: ctx,
	}

	// Create the source iterator. We randomize the order we visit nodes
	// to reduce collisions between schedulers and to do a basic load
	// balancing across eligible nodes.
	source := NewRandomIterator(ctx, baseNodes)

	// Attach the job constraints. The job is filled in later.
	stack.jobConstraint = NewConstraintIterator(ctx, source, nil)

	// Filter on task group drivers first as they are faster
	stack.taskGroupDrivers = NewDriverIterator(ctx, stack.jobConstraint, nil)

	// Filter on task group constraints second
	stack.taskGroupConstraint = NewConstraintIterator(ctx, stack.taskGroupDrivers, nil)

	// Upgrade from feasible to rank iterator
	rankSource := NewFeasibleRankIterator(ctx, stack.taskGroupConstraint)

	// Apply the bin packing, this depends on the resources needed by a particular task group.
	stack.binPack = NewBinPackIterator(ctx, rankSource, nil, true, 0)

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
	limitIter := NewLimitIterator(ctx, stack.binPack, limit)

	// Select the node with the maximum score for placement
	stack.maxScore = NewMaxScoreIterator(ctx, limitIter)
	return stack
}

func (s *ServiceStack) SetJob(job *structs.Job) {
	s.jobConstraint.SetConstraints(job.Constraints)
	s.binPack.SetPriority(job.Priority)
}

func (s *ServiceStack) Select(tg *structs.TaskGroup) (*RankedNode, *structs.Resources) {
	// Reset the max selector and context
	s.maxScore.Reset()
	s.ctx.Reset()

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

	// Update the parameters of iterators
	s.taskGroupDrivers.SetDrivers(drivers)
	s.taskGroupConstraint.SetConstraints(constr)
	s.binPack.SetResources(size)

	// Return the node with the max score
	return s.maxScore.Next(), size
}

package api

// Jobs is used to access the job-specific endpoints.
type Jobs struct {
	client *Client
}

// Jobs returns a handle on the jobs endpoints.
func (c *Client) Jobs() *Jobs {
	return &Jobs{client: c}
}

// Register is used to register a new job. It returns the ID
// of the evaluation, along with any errors encountered.
func (j *Jobs) Register(job *Job, q *WriteOptions) (string, *WriteMeta, error) {
	var resp registerJobResponse

	req := &registerJobRequest{job}
	wm, err := j.client.write("/v1/jobs", req, &resp, q)
	if err != nil {
		return "", nil, err
	}
	return resp.EvalID, wm, nil
}

// List is used to list all of the existing jobs.
func (j *Jobs) List() ([]*Job, *QueryMeta, error) {
	var resp []*Job
	qm, err := j.client.query("/v1/jobs", &resp, nil)
	if err != nil {
		return nil, qm, err
	}
	return resp, qm, nil
}

// Info is used to retrieve information about a particular
// job given its unique ID.
func (j *Jobs) Info(jobID string) (*Job, *QueryMeta, error) {
	var resp Job
	qm, err := j.client.query("/v1/job/"+jobID, &resp, nil)
	if err != nil {
		return nil, nil, err
	}
	return &resp, qm, nil
}

// Allocations is used to return the allocs for a given job ID.
func (j *Jobs) Allocations(jobID string) ([]*Allocation, *QueryMeta, error) {
	var resp []*Allocation
	qm, err := j.client.query("/v1/job/"+jobID+"/allocations", &resp, nil)
	if err != nil {
		return nil, nil, err
	}
	return resp, qm, nil
}

// Evaluations is used to query the evaluations associated with
// the given job ID.
func (j *Jobs) Evaluations(jobID string) ([]*Evaluation, *QueryMeta, error) {
	var resp []*Evaluation
	qm, err := j.client.query("/v1/job/"+jobID+"/evaluations", &resp, nil)
	if err != nil {
		return nil, nil, err
	}
	return resp, qm, nil
}

// Delete is used to remove an existing job.
func (j *Jobs) Delete(jobID string, q *WriteOptions) (*WriteMeta, error) {
	wm, err := j.client.delete("/v1/job/"+jobID, nil, q)
	if err != nil {
		return nil, err
	}
	return wm, nil
}

// Job is used to serialize a job.
type Job struct {
	ID                string
	Name              string
	Type              string
	Priority          int
	AllAtOnce         bool
	Datacenters       []string
	Constraints       []*Constraint
	Meta              map[string]string
	Status            string
	StatusDescription string
}

// registerJobRequest is used to serialize a job registration
type registerJobRequest struct {
	Job *Job
}

// registerJobResponse is used to deserialize a job response
type registerJobResponse struct {
	EvalID string
}

// Constraint is used to serialize a job placement constraint.
type Constraint struct {
	Hard    bool
	LTarget string
	RTarget string
	Operand string
	Weight  int
}

// HardConstraint is used to create a new hard constraint.
func HardConstraint(left, operand, right string) *Constraint {
	return constraint(left, operand, right, true, 0)
}

// SoftConstraint is used to create a new soft constraint. It
// takes an additional weight parameter to allow balancing
// multiple soft constraints amongst eachother.
func SoftConstraint(left, operand, right string, weight int) *Constraint {
	return constraint(left, operand, right, false, weight)
}

// constraint generates a new job placement constraint.
func constraint(left, operand, right string, hard bool, weight int) *Constraint {
	return &Constraint{
		Hard:    hard,
		LTarget: left,
		RTarget: right,
		Operand: operand,
		Weight:  weight,
	}
}

// TaskGroup is the unit of scheduling.
type TaskGroup struct {
	Name        string
	Count       int
	Constraints []*Constraint
	Tasks       []*Task
	Meta        map[string]string
}

// NewTaskGroup creates a new TaskGroup.
func NewTaskGroup(name string, count int) *TaskGroup {
	return &TaskGroup{
		Name:  name,
		Count: count,
	}
}

// Constrain is used to add a constraint to a task group.
func (g *TaskGroup) Constrain(c *Constraint) *TaskGroup {
	g.Constraints = append(g.Constraints, c)
	return g
}

// AddMeta is used to add a meta k/v pair to a task group
func (g *TaskGroup) AddMeta(key, val string) *TaskGroup {
	if g.Meta == nil {
		g.Meta = make(map[string]string)
	}
	g.Meta[key] = val
	return g
}

// AddTask is used to add a new task to a task group.
func (g *TaskGroup) AddTask(t *Task) *TaskGroup {
	g.Tasks = append(g.Tasks, t)
	return g
}

// Task is a single process in a task group.
type Task struct {
	Name        string
	Driver      string
	Config      map[string]string
	Constraints []*Constraint
	Resources   *Resources
	Meta        map[string]string
}

// NewTask creates and initializes a new Task.
func NewTask(name, driver string) *Task {
	return &Task{
		Name:   name,
		Driver: driver,
	}
}

// Configure is used to configure a single k/v pair on
// the task.
func (t *Task) Configure(key, val string) *Task {
	if t.Config == nil {
		t.Config = make(map[string]string)
	}
	t.Config[key] = val
	return t
}

// AddMeta is used to add metadata k/v pairs to the task.
func (t *Task) AddMeta(key, val string) *Task {
	if t.Meta == nil {
		t.Meta = make(map[string]string)
	}
	t.Meta[key] = val
	return t
}

// Require is used to add resource requirements to a task.
// It creates and initializes the task resources.
func (t *Task) Require(r *Resources) *Task {
	if t.Resources == nil {
		t.Resources = &Resources{}
	}
	if r == nil {
		return t
	}
	t.Resources.CPU += r.CPU
	t.Resources.MemoryMB += r.MemoryMB
	t.Resources.DiskMB += r.DiskMB
	t.Resources.IOPS += r.IOPS
	return t
}

// Resources encapsulates the required resources of
// a given task or task group.
type Resources struct {
	CPU      float64
	MemoryMB int
	DiskMB   int
	IOPS     int
	Networks []*NetworkResource
}

// NetworkResource is used to describe required network
// resources of a given task.
type NetworkResource struct {
	Public        bool
	CIDR          string
	ReservedPorts []int
	MBits         int
}

package api

import (
	"fmt"
	"sort"
	"time"
)

const (
	// JobTypeService indicates a long-running processes
	JobTypeService = "service"

	// JobTypeBatch indicates a short-lived process
	JobTypeBatch = "batch"
)

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

	req := &RegisterJobRequest{job}
	wm, err := j.client.write("/v1/jobs", req, &resp, q)
	if err != nil {
		return "", nil, err
	}
	return resp.EvalID, wm, nil
}

// List is used to list all of the existing jobs.
func (j *Jobs) List(q *QueryOptions) ([]*JobListStub, *QueryMeta, error) {
	var resp []*JobListStub
	qm, err := j.client.query("/v1/jobs", &resp, q)
	if err != nil {
		return nil, qm, err
	}
	sort.Sort(JobIDSort(resp))
	return resp, qm, nil
}

// PrefixList is used to list all existing jobs that match the prefix.
func (j *Jobs) PrefixList(prefix string) ([]*JobListStub, *QueryMeta, error) {
	return j.List(&QueryOptions{Prefix: prefix})
}

// Info is used to retrieve information about a particular
// job given its unique ID.
func (j *Jobs) Info(jobID string, q *QueryOptions) (*Job, *QueryMeta, error) {
	var resp Job
	qm, err := j.client.query("/v1/job/"+jobID, &resp, q)
	if err != nil {
		return nil, nil, err
	}
	return &resp, qm, nil
}

// Status retrieves the current state of a particular job given its unique ID.
func (j *Jobs) Status(jobID string, q *QueryOptions) (*JobStatus, *QueryMeta, error) {
	var resp JobStatus
	qm, err := j.client.query("/v1/job/"+jobID+"/status", &resp, q)
	if err != nil {
		return nil, nil, err
	}
	return &resp, qm, nil
}

// Allocations is used to return the allocs for a given job ID.
func (j *Jobs) Allocations(jobID string, q *QueryOptions) ([]*AllocationListStub, *QueryMeta, error) {
	var resp []*AllocationListStub
	qm, err := j.client.query("/v1/job/"+jobID+"/allocations", &resp, q)
	if err != nil {
		return nil, nil, err
	}
	sort.Sort(AllocIndexSort(resp))
	return resp, qm, nil
}

// Evaluations is used to query the evaluations associated with
// the given job ID.
func (j *Jobs) Evaluations(jobID string, q *QueryOptions) ([]*Evaluation, *QueryMeta, error) {
	var resp []*Evaluation
	qm, err := j.client.query("/v1/job/"+jobID+"/evaluations", &resp, q)
	if err != nil {
		return nil, nil, err
	}
	sort.Sort(EvalIndexSort(resp))
	return resp, qm, nil
}

// Deregister is used to remove an existing job.
func (j *Jobs) Deregister(jobID string, q *WriteOptions) (string, *WriteMeta, error) {
	var resp deregisterJobResponse
	wm, err := j.client.delete("/v1/job/"+jobID, &resp, q)
	if err != nil {
		return "", nil, err
	}
	return resp.EvalID, wm, nil
}

// ForceEvaluate is used to force-evaluate an existing job.
func (j *Jobs) ForceEvaluate(jobID string, q *WriteOptions) (string, *WriteMeta, error) {
	var resp registerJobResponse
	wm, err := j.client.write("/v1/job/"+jobID+"/evaluate", nil, &resp, q)
	if err != nil {
		return "", nil, err
	}
	return resp.EvalID, wm, nil
}

// PeriodicForce spawns a new instance of the periodic job and returns the eval ID
func (j *Jobs) PeriodicForce(jobID string, q *WriteOptions) (string, *WriteMeta, error) {
	var resp periodicForceResponse
	wm, err := j.client.write("/v1/job/"+jobID+"/periodic/force", nil, &resp, q)
	if err != nil {
		return "", nil, err
	}
	return resp.EvalID, wm, nil
}

func (j *Jobs) Plan(job *Job, diff bool, q *WriteOptions) (*JobPlanResponse, *WriteMeta, error) {
	if job == nil {
		return nil, nil, fmt.Errorf("must pass non-nil job")
	}

	var resp JobPlanResponse
	req := &JobPlanRequest{
		Job:  job,
		Diff: diff,
	}
	wm, err := j.client.write("/v1/job/"+job.ID+"/plan", req, &resp, q)
	if err != nil {
		return nil, nil, err
	}

	return &resp, wm, nil
}

// periodicForceResponse is used to deserialize a force response
type periodicForceResponse struct {
	EvalID string
}

// UpdateStrategy is for serializing update strategy for a job.
type UpdateStrategy struct {
	Stagger     time.Duration
	MaxParallel int
}

// PeriodicConfig is for serializing periodic config for a job.
type PeriodicConfig struct {
	Enabled         bool
	Spec            string
	SpecType        string
	ProhibitOverlap bool
}

// Job is used to serialize a job.
type Job struct {
	Region            string
	ID                string
	Name              string
	Type              string
	Priority          int
	AllAtOnce         bool
	Datacenters       []string
	Constraints       []*Constraint
	TaskGroups        []*TaskGroup
	Update            *UpdateStrategy
	Periodic          *PeriodicConfig
	Meta              map[string]string
	Status            string
	StatusDescription string
	CreateIndex       uint64
	ModifyIndex       uint64
}

// JobListStub is used to return a subset of information about
// jobs during list operations.
type JobListStub struct {
	ID                string
	ParentID          string
	Name              string
	Type              string
	Priority          int
	Periodic          bool
	Status            string
	StatusDescription string
	CreateIndex       uint64
	ModifyIndex       uint64
}

// JobIDSort is used to sort jobs by their job ID's.
type JobIDSort []*JobListStub

func (j JobIDSort) Len() int {
	return len(j)
}

func (j JobIDSort) Less(a, b int) bool {
	return j[a].ID < j[b].ID
}

func (j JobIDSort) Swap(a, b int) {
	j[a], j[b] = j[b], j[a]
}

// NewServiceJob creates and returns a new service-style job
// for long-lived processes using the provided name, ID, and
// relative job priority.
func NewServiceJob(id, name, region string, pri int) *Job {
	return newJob(id, name, region, JobTypeService, pri)
}

// NewBatchJob creates and returns a new batch-style job for
// short-lived processes using the provided name and ID along
// with the relative job priority.
func NewBatchJob(id, name, region string, pri int) *Job {
	return newJob(id, name, region, JobTypeBatch, pri)
}

// newJob is used to create a new Job struct.
func newJob(id, name, region, typ string, pri int) *Job {
	return &Job{
		Region:   region,
		ID:       id,
		Name:     name,
		Type:     typ,
		Priority: pri,
	}
}

// SetMeta is used to set arbitrary k/v pairs of metadata on a job.
func (j *Job) SetMeta(key, val string) *Job {
	if j.Meta == nil {
		j.Meta = make(map[string]string)
	}
	j.Meta[key] = val
	return j
}

// AddDatacenter is used to add a datacenter to a job.
func (j *Job) AddDatacenter(dc string) *Job {
	j.Datacenters = append(j.Datacenters, dc)
	return j
}

// Constrain is used to add a constraint to a job.
func (j *Job) Constrain(c *Constraint) *Job {
	j.Constraints = append(j.Constraints, c)
	return j
}

// AddTaskGroup adds a task group to an existing job.
func (j *Job) AddTaskGroup(grp *TaskGroup) *Job {
	j.TaskGroups = append(j.TaskGroups, grp)
	return j
}

// AddPeriodicConfig adds a periodic config to an existing job.
func (j *Job) AddPeriodicConfig(cfg *PeriodicConfig) *Job {
	j.Periodic = cfg
	return j
}

// RegisterJobRequest is used to serialize a job registration
type RegisterJobRequest struct {
	Job *Job
}

// registerJobResponse is used to deserialize a job response
type registerJobResponse struct {
	EvalID string
}

// deregisterJobResponse is used to decode a deregister response
type deregisterJobResponse struct {
	EvalID string
}

type JobPlanRequest struct {
	Job  *Job
	Diff bool
}

type JobPlanResponse struct {
	JobModifyIndex uint64
	CreatedEvals   []*Evaluation
	Diff           *JobDiff
	Annotations    *PlanAnnotations
}

type JobDiff struct {
	Type       string
	ID         string
	Fields     []*FieldDiff
	Objects    []*ObjectDiff
	TaskGroups []*TaskGroupDiff
}

type TaskGroupDiff struct {
	Type    string
	Name    string
	Fields  []*FieldDiff
	Objects []*ObjectDiff
	Tasks   []*TaskDiff
	Updates map[string]uint64
}

type TaskDiff struct {
	Type        string
	Name        string
	Fields      []*FieldDiff
	Objects     []*ObjectDiff
	Annotations []string
}

type FieldDiff struct {
	Type        string
	Name        string
	Old, New    string
	Annotations []string
}

type ObjectDiff struct {
	Type    string
	Name    string
	Fields  []*FieldDiff
	Objects []*ObjectDiff
}

type PlanAnnotations struct {
	DesiredTGUpdates map[string]*DesiredUpdates
}

type DesiredUpdates struct {
	Ignore            uint64
	Place             uint64
	Migrate           uint64
	Stop              uint64
	InPlaceUpdate     uint64
	DestructiveUpdate uint64
}

type JobStatus struct {
	AllocStateCounts
	TaskGroups map[string]AllocStateCounts
	Status     string
}

type AllocStateCounts struct {
	Pending  uint64
	Starting uint64
	Running  uint64
	Complete uint64
	Failed   uint64
}

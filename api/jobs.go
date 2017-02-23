package api

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"time"

	"github.com/gorhill/cronexpr"
	"github.com/hashicorp/nomad/helper"
)

const (
	// JobTypeService indicates a long-running processes
	JobTypeService = "service"

	// JobTypeBatch indicates a short-lived process
	JobTypeBatch = "batch"

	// PeriodicSpecCron is used for a cron spec.
	PeriodicSpecCron = "cron"
)

const (
	// RegisterEnforceIndexErrPrefix is the prefix to use in errors caused by
	// enforcing the job modify index during registers.
	RegisterEnforceIndexErrPrefix = "Enforcing job modify index"
)

// Jobs is used to access the job-specific endpoints.
type Jobs struct {
	client *Client
}

// Jobs returns a handle on the jobs endpoints.
func (c *Client) Jobs() *Jobs {
	return &Jobs{client: c}
}

func (j *Jobs) Validate(job *Job, q *WriteOptions) (*JobValidateResponse, *WriteMeta, error) {
	var resp JobValidateResponse
	req := &JobValidateRequest{Job: job}
	if q != nil {
		req.WriteRequest = WriteRequest{Region: q.Region}
	}
	wm, err := j.client.write("/v1/validate/job", req, &resp, q)
	return &resp, wm, err
}

// Register is used to register a new job. It returns the ID
// of the evaluation, along with any errors encountered.
func (j *Jobs) Register(job *Job, q *WriteOptions) (string, *WriteMeta, error) {

	var resp registerJobResponse

	req := &RegisterJobRequest{Job: job}
	wm, err := j.client.write("/v1/jobs", req, &resp, q)
	if err != nil {
		return "", nil, err
	}
	return resp.EvalID, wm, nil
}

// EnforceRegister is used to register a job enforcing its job modify index.
func (j *Jobs) EnforceRegister(job *Job, modifyIndex uint64, q *WriteOptions) (string, *WriteMeta, error) {

	var resp registerJobResponse

	req := &RegisterJobRequest{
		Job:            job,
		EnforceIndex:   true,
		JobModifyIndex: modifyIndex,
	}
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

// Allocations is used to return the allocs for a given job ID.
func (j *Jobs) Allocations(jobID string, allAllocs bool, q *QueryOptions) ([]*AllocationListStub, *QueryMeta, error) {
	var resp []*AllocationListStub
	u, err := url.Parse("/v1/job/" + jobID + "/allocations")
	if err != nil {
		return nil, nil, err
	}

	v := u.Query()
	v.Add("all", strconv.FormatBool(allAllocs))
	u.RawQuery = v.Encode()

	qm, err := j.client.query(u.String(), &resp, q)
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
	wm, err := j.client.write("/v1/job/"+*job.ID+"/plan", req, &resp, q)
	if err != nil {
		return nil, nil, err
	}

	return &resp, wm, nil
}

func (j *Jobs) Summary(jobID string, q *QueryOptions) (*JobSummary, *QueryMeta, error) {
	var resp JobSummary
	qm, err := j.client.query("/v1/job/"+jobID+"/summary", &resp, q)
	if err != nil {
		return nil, nil, err
	}
	return &resp, qm, nil
}

func (j *Jobs) Dispatch(jobID string, meta map[string]string,
	payload []byte, q *WriteOptions) (*JobDispatchResponse, *WriteMeta, error) {
	var resp JobDispatchResponse
	req := &JobDispatchRequest{
		JobID:   jobID,
		Meta:    meta,
		Payload: payload,
	}
	wm, err := j.client.write("/v1/job/"+jobID+"/dispatch", req, &resp, q)
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
	MaxParallel int `mapstructure:"max_parallel"`
}

// PeriodicConfig is for serializing periodic config for a job.
type PeriodicConfig struct {
	Enabled         *bool
	Spec            *string
	SpecType        *string
	ProhibitOverlap *bool   `mapstructure:"prohibit_overlap"`
	TimeZone        *string `mapstructure:"time_zone"`
}

func (p *PeriodicConfig) Canonicalize() {
	if p.Enabled == nil {
		p.Enabled = helper.BoolToPtr(true)
	}
	if p.SpecType == nil {
		p.SpecType = helper.StringToPtr(PeriodicSpecCron)
	}
	if p.ProhibitOverlap == nil {
		p.ProhibitOverlap = helper.BoolToPtr(false)
	}
	if p.TimeZone == nil || *p.TimeZone == "" {
		p.TimeZone = helper.StringToPtr("UTC")
	}
}

// Next returns the closest time instant matching the spec that is after the
// passed time. If no matching instance exists, the zero value of time.Time is
// returned. The `time.Location` of the returned value matches that of the
// passed time.
func (p *PeriodicConfig) Next(fromTime time.Time) time.Time {
	if *p.SpecType == PeriodicSpecCron {
		if e, err := cronexpr.Parse(*p.Spec); err == nil {
			return e.Next(fromTime)
		}
	}

	return time.Time{}
}

func (p *PeriodicConfig) GetLocation() (*time.Location, error) {
	if p.TimeZone == nil || *p.TimeZone == "" {
		return time.UTC, nil
	}

	return time.LoadLocation(*p.TimeZone)
}

// ParameterizedJobConfig is used to configure the parameterized job.
type ParameterizedJobConfig struct {
	Payload      string
	MetaRequired []string `mapstructure:"meta_required"`
	MetaOptional []string `mapstructure:"meta_optional"`
}

// Job is used to serialize a job.
type Job struct {
	Region            *string
	ID                *string
	ParentID          *string
	Name              *string
	Type              *string
	Priority          *int
	AllAtOnce         *bool `mapstructure:"all_at_once"`
	Datacenters       []string
	Constraints       []*Constraint
	TaskGroups        []*TaskGroup
	Update            *UpdateStrategy
	Periodic          *PeriodicConfig
	ParameterizedJob  *ParameterizedJobConfig
	Payload           []byte
	Meta              map[string]string
	VaultToken        *string `mapstructure:"vault_token"`
	Status            *string
	StatusDescription *string
	CreateIndex       *uint64
	ModifyIndex       *uint64
	JobModifyIndex    *uint64
}

// IsPeriodic returns whether a job is periodic.
func (j *Job) IsPeriodic() bool {
	return j.Periodic != nil
}

// IsParameterized returns whether a job is parameterized job.
func (j *Job) IsParameterized() bool {
	return j.ParameterizedJob != nil
}

func (j *Job) Canonicalize() {
	if j.ID == nil {
		j.ID = helper.StringToPtr("")
	}
	if j.Name == nil {
		j.Name = j.ID
	}
	if j.ParentID == nil {
		j.ParentID = helper.StringToPtr("")
	}
	if j.Priority == nil {
		j.Priority = helper.IntToPtr(50)
	}
	if j.Region == nil {
		j.Region = helper.StringToPtr("global")
	}
	if j.Type == nil {
		j.Type = helper.StringToPtr("service")
	}
	if j.AllAtOnce == nil {
		j.AllAtOnce = helper.BoolToPtr(false)
	}
	if j.VaultToken == nil {
		j.VaultToken = helper.StringToPtr("")
	}
	if j.Status == nil {
		j.Status = helper.StringToPtr("")
	}
	if j.StatusDescription == nil {
		j.StatusDescription = helper.StringToPtr("")
	}
	if j.CreateIndex == nil {
		j.CreateIndex = helper.Uint64ToPtr(0)
	}
	if j.ModifyIndex == nil {
		j.ModifyIndex = helper.Uint64ToPtr(0)
	}
	if j.JobModifyIndex == nil {
		j.JobModifyIndex = helper.Uint64ToPtr(0)
	}
	if j.Periodic != nil {
		j.Periodic.Canonicalize()
	}

	for _, tg := range j.TaskGroups {
		tg.Canonicalize(j)
	}
}

// JobSummary summarizes the state of the allocations of a job
type JobSummary struct {
	JobID    string
	Summary  map[string]TaskGroupSummary
	Children *JobChildrenSummary

	// Raft Indexes
	CreateIndex uint64
	ModifyIndex uint64
}

// JobChildrenSummary contains the summary of children job status
type JobChildrenSummary struct {
	Pending int64
	Running int64
	Dead    int64
}

func (jc *JobChildrenSummary) Sum() int {
	if jc == nil {
		return 0
	}

	return int(jc.Pending + jc.Running + jc.Dead)
}

// TaskGroup summarizes the state of all the allocations of a particular
// TaskGroup
type TaskGroupSummary struct {
	Queued   int
	Complete int
	Failed   int
	Running  int
	Starting int
	Lost     int
}

// JobListStub is used to return a subset of information about
// jobs during list operations.
type JobListStub struct {
	ID                string
	ParentID          string
	Name              string
	Type              string
	Priority          int
	Status            string
	StatusDescription string
	JobSummary        *JobSummary
	CreateIndex       uint64
	ModifyIndex       uint64
	JobModifyIndex    uint64
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
		Region:   &region,
		ID:       &id,
		Name:     &name,
		Type:     &typ,
		Priority: &pri,
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

type WriteRequest struct {
	// The target region for this write
	Region string
}

// JobValidateRequest is used to validate a job
type JobValidateRequest struct {
	Job *Job
	WriteRequest
}

// JobValidateResponse is the response from validate request
type JobValidateResponse struct {
	// DriverConfigValidated indicates whether the agent validated the driver
	// config
	DriverConfigValidated bool

	// ValidationErrors is a list of validation errors
	ValidationErrors []string
}

// JobUpdateRequest is used to update a job
type JobRegisterRequest struct {
	Job *Job
	// If EnforceIndex is set then the job will only be registered if the passed
	// JobModifyIndex matches the current Jobs index. If the index is zero, the
	// register only occurs if the job is new.
	EnforceIndex   bool
	JobModifyIndex uint64

	WriteRequest
}

// RegisterJobRequest is used to serialize a job registration
type RegisterJobRequest struct {
	Job            *Job
	EnforceIndex   bool   `json:",omitempty"`
	JobModifyIndex uint64 `json:",omitempty"`
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
	WriteRequest
}

type JobPlanResponse struct {
	JobModifyIndex     uint64
	CreatedEvals       []*Evaluation
	Diff               *JobDiff
	Annotations        *PlanAnnotations
	FailedTGAllocs     map[string]*AllocationMetric
	NextPeriodicLaunch time.Time
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

type JobDispatchRequest struct {
	JobID   string
	Payload []byte
	Meta    map[string]string
}

type JobDispatchResponse struct {
	DispatchedJobID string
	EvalID          string
	EvalCreateIndex uint64
	JobCreateIndex  uint64
	WriteMeta
}

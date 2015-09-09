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

// Job is used to serialize a job.
type Job struct {
	ID                string
	Name              string
	Type              string
	Priority          int
	AllAtOnce         bool
	Datacenters       []string
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

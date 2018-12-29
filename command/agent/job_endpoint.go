package agent

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/golang/snappy"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/jobspec"
	"github.com/hashicorp/nomad/nomad/structs"
)

func (s *HTTPServer) JobsRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	switch req.Method {
	case "GET":
		return s.jobListRequest(resp, req)
	case "PUT", "POST":
		return s.jobUpdate(resp, req, "")
	default:
		return nil, CodedError(405, ErrInvalidMethod)
	}
}

func (s *HTTPServer) jobListRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	args := structs.JobListRequest{}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.JobListResponse
	if err := s.agent.RPC("Job.List", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.Jobs == nil {
		out.Jobs = make([]*structs.JobListStub, 0)
	}
	return out.Jobs, nil
}

func (s *HTTPServer) JobSpecificRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	path := strings.TrimPrefix(req.URL.Path, "/v1/job/")
	switch {
	case strings.HasSuffix(path, "/evaluate"):
		jobName := strings.TrimSuffix(path, "/evaluate")
		return s.jobForceEvaluate(resp, req, jobName)
	case strings.HasSuffix(path, "/allocations"):
		jobName := strings.TrimSuffix(path, "/allocations")
		return s.jobAllocations(resp, req, jobName)
	case strings.HasSuffix(path, "/evaluations"):
		jobName := strings.TrimSuffix(path, "/evaluations")
		return s.jobEvaluations(resp, req, jobName)
	case strings.HasSuffix(path, "/periodic/force"):
		jobName := strings.TrimSuffix(path, "/periodic/force")
		return s.periodicForceRequest(resp, req, jobName)
	case strings.HasSuffix(path, "/plan"):
		jobName := strings.TrimSuffix(path, "/plan")
		return s.jobPlan(resp, req, jobName)
	case strings.HasSuffix(path, "/summary"):
		jobName := strings.TrimSuffix(path, "/summary")
		return s.jobSummaryRequest(resp, req, jobName)
	case strings.HasSuffix(path, "/dispatch"):
		jobName := strings.TrimSuffix(path, "/dispatch")
		return s.jobDispatchRequest(resp, req, jobName)
	case strings.HasSuffix(path, "/versions"):
		jobName := strings.TrimSuffix(path, "/versions")
		return s.jobVersions(resp, req, jobName)
	case strings.HasSuffix(path, "/revert"):
		jobName := strings.TrimSuffix(path, "/revert")
		return s.jobRevert(resp, req, jobName)
	case strings.HasSuffix(path, "/deployments"):
		jobName := strings.TrimSuffix(path, "/deployments")
		return s.jobDeployments(resp, req, jobName)
	case strings.HasSuffix(path, "/deployment"):
		jobName := strings.TrimSuffix(path, "/deployment")
		return s.jobLatestDeployment(resp, req, jobName)
	case strings.HasSuffix(path, "/stable"):
		jobName := strings.TrimSuffix(path, "/stable")
		return s.jobStable(resp, req, jobName)
	default:
		return s.jobCRUD(resp, req, path)
	}
}

func (s *HTTPServer) jobForceEvaluate(resp http.ResponseWriter, req *http.Request,
	jobName string) (interface{}, error) {
	if req.Method != "PUT" && req.Method != "POST" {
		return nil, CodedError(405, ErrInvalidMethod)
	}
	var args structs.JobEvaluateRequest

	// TODO(preetha): remove in 0.9
	// COMPAT: For backwards compatibility allow using this endpoint without a payload
	if req.ContentLength == 0 {
		args = structs.JobEvaluateRequest{
			JobID: jobName,
		}
	} else {
		if err := decodeBody(req, &args); err != nil {
			return nil, CodedError(400, err.Error())
		}
		if args.JobID == "" {
			return nil, CodedError(400, "Job ID must be specified")
		}

		if jobName != "" && args.JobID != jobName {
			return nil, CodedError(400, "JobID not same as job name")
		}
	}
	s.parseWriteRequest(req, &args.WriteRequest)

	var out structs.JobRegisterResponse
	if err := s.agent.RPC("Job.Evaluate", &args, &out); err != nil {
		return nil, err
	}
	setIndex(resp, out.Index)
	return out, nil
}

func (s *HTTPServer) jobPlan(resp http.ResponseWriter, req *http.Request,
	jobName string) (interface{}, error) {
	if req.Method != "PUT" && req.Method != "POST" {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	var args api.JobPlanRequest
	if err := decodeBody(req, &args); err != nil {
		return nil, CodedError(400, err.Error())
	}
	if args.Job == nil {
		return nil, CodedError(400, "Job must be specified")
	}
	if args.Job.ID == nil {
		return nil, CodedError(400, "Job must have a valid ID")
	}
	if jobName != "" && *args.Job.ID != jobName {
		return nil, CodedError(400, "Job ID does not match")
	}

	sJob := ApiJobToStructJob(args.Job)
	planReq := structs.JobPlanRequest{
		Job:            sJob,
		Diff:           args.Diff,
		PolicyOverride: args.PolicyOverride,
		WriteRequest: structs.WriteRequest{
			Region: args.WriteRequest.Region,
		},
	}
	s.parseWriteRequest(req, &planReq.WriteRequest)
	planReq.Namespace = sJob.Namespace

	var out structs.JobPlanResponse
	if err := s.agent.RPC("Job.Plan", &planReq, &out); err != nil {
		return nil, err
	}
	setIndex(resp, out.Index)
	return out, nil
}

func (s *HTTPServer) ValidateJobRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Ensure request method is POST or PUT
	if !(req.Method == "POST" || req.Method == "PUT") {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	var validateRequest api.JobValidateRequest
	if err := decodeBody(req, &validateRequest); err != nil {
		return nil, CodedError(400, err.Error())
	}
	if validateRequest.Job == nil {
		return nil, CodedError(400, "Job must be specified")
	}

	job := ApiJobToStructJob(validateRequest.Job)
	args := structs.JobValidateRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region: validateRequest.Region,
		},
	}
	s.parseWriteRequest(req, &args.WriteRequest)
	args.Namespace = job.Namespace

	var out structs.JobValidateResponse
	if err := s.agent.RPC("Job.Validate", &args, &out); err != nil {
		return nil, err
	}

	return out, nil
}

func (s *HTTPServer) periodicForceRequest(resp http.ResponseWriter, req *http.Request,
	jobName string) (interface{}, error) {
	if req.Method != "PUT" && req.Method != "POST" {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	args := structs.PeriodicForceRequest{
		JobID: jobName,
	}
	s.parseWriteRequest(req, &args.WriteRequest)

	var out structs.PeriodicForceResponse
	if err := s.agent.RPC("Periodic.Force", &args, &out); err != nil {
		return nil, err
	}
	setIndex(resp, out.Index)
	return out, nil
}

func (s *HTTPServer) jobAllocations(resp http.ResponseWriter, req *http.Request,
	jobName string) (interface{}, error) {
	if req.Method != "GET" {
		return nil, CodedError(405, ErrInvalidMethod)
	}
	allAllocs, _ := strconv.ParseBool(req.URL.Query().Get("all"))

	args := structs.JobSpecificRequest{
		JobID:     jobName,
		AllAllocs: allAllocs,
	}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.JobAllocationsResponse
	if err := s.agent.RPC("Job.Allocations", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.Allocations == nil {
		out.Allocations = make([]*structs.AllocListStub, 0)
	}
	for _, alloc := range out.Allocations {
		alloc.SetEventDisplayMessages()
	}
	return out.Allocations, nil
}

func (s *HTTPServer) jobEvaluations(resp http.ResponseWriter, req *http.Request,
	jobName string) (interface{}, error) {
	if req.Method != "GET" {
		return nil, CodedError(405, ErrInvalidMethod)
	}
	args := structs.JobSpecificRequest{
		JobID: jobName,
	}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.JobEvaluationsResponse
	if err := s.agent.RPC("Job.Evaluations", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.Evaluations == nil {
		out.Evaluations = make([]*structs.Evaluation, 0)
	}
	return out.Evaluations, nil
}

func (s *HTTPServer) jobDeployments(resp http.ResponseWriter, req *http.Request,
	jobName string) (interface{}, error) {
	if req.Method != "GET" {
		return nil, CodedError(405, ErrInvalidMethod)
	}
	args := structs.JobSpecificRequest{
		JobID: jobName,
	}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.DeploymentListResponse
	if err := s.agent.RPC("Job.Deployments", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.Deployments == nil {
		out.Deployments = make([]*structs.Deployment, 0)
	}
	return out.Deployments, nil
}

func (s *HTTPServer) jobLatestDeployment(resp http.ResponseWriter, req *http.Request,
	jobName string) (interface{}, error) {
	if req.Method != "GET" {
		return nil, CodedError(405, ErrInvalidMethod)
	}
	args := structs.JobSpecificRequest{
		JobID: jobName,
	}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.SingleDeploymentResponse
	if err := s.agent.RPC("Job.LatestDeployment", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	return out.Deployment, nil
}

func (s *HTTPServer) jobCRUD(resp http.ResponseWriter, req *http.Request,
	jobName string) (interface{}, error) {
	switch req.Method {
	case "GET":
		return s.jobQuery(resp, req, jobName)
	case "PUT", "POST":
		return s.jobUpdate(resp, req, jobName)
	case "DELETE":
		return s.jobDelete(resp, req, jobName)
	default:
		return nil, CodedError(405, ErrInvalidMethod)
	}
}

func (s *HTTPServer) jobQuery(resp http.ResponseWriter, req *http.Request,
	jobName string) (interface{}, error) {
	args := structs.JobSpecificRequest{
		JobID: jobName,
	}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.SingleJobResponse
	if err := s.agent.RPC("Job.GetJob", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.Job == nil {
		return nil, CodedError(404, "job not found")
	}

	// Decode the payload if there is any
	job := out.Job
	if len(job.Payload) != 0 {
		decoded, err := snappy.Decode(nil, out.Job.Payload)
		if err != nil {
			return nil, err
		}
		job = job.Copy()
		job.Payload = decoded
	}

	return job, nil
}

func (s *HTTPServer) jobUpdate(resp http.ResponseWriter, req *http.Request,
	jobName string) (interface{}, error) {
	var args api.JobRegisterRequest
	if err := decodeBody(req, &args); err != nil {
		return nil, CodedError(400, err.Error())
	}
	if args.Job == nil {
		return nil, CodedError(400, "Job must be specified")
	}

	if args.Job.ID == nil {
		return nil, CodedError(400, "Job ID hasn't been provided")
	}
	if jobName != "" && *args.Job.ID != jobName {
		return nil, CodedError(400, "Job ID does not match name")
	}

	sJob := ApiJobToStructJob(args.Job)

	regReq := structs.JobRegisterRequest{
		Job:            sJob,
		EnforceIndex:   args.EnforceIndex,
		JobModifyIndex: args.JobModifyIndex,
		PolicyOverride: args.PolicyOverride,
		WriteRequest: structs.WriteRequest{
			Region:    args.WriteRequest.Region,
			AuthToken: args.WriteRequest.SecretID,
		},
	}
	s.parseWriteRequest(req, &regReq.WriteRequest)
	regReq.Namespace = sJob.Namespace

	var out structs.JobRegisterResponse
	if err := s.agent.RPC("Job.Register", &regReq, &out); err != nil {
		return nil, err
	}
	setIndex(resp, out.Index)
	return out, nil
}

func (s *HTTPServer) jobDelete(resp http.ResponseWriter, req *http.Request,
	jobName string) (interface{}, error) {

	purgeStr := req.URL.Query().Get("purge")
	var purgeBool bool
	if purgeStr != "" {
		var err error
		purgeBool, err = strconv.ParseBool(purgeStr)
		if err != nil {
			return nil, fmt.Errorf("Failed to parse value of %q (%v) as a bool: %v", "purge", purgeStr, err)
		}
	}

	args := structs.JobDeregisterRequest{
		JobID: jobName,
		Purge: purgeBool,
	}
	s.parseWriteRequest(req, &args.WriteRequest)

	var out structs.JobDeregisterResponse
	if err := s.agent.RPC("Job.Deregister", &args, &out); err != nil {
		return nil, err
	}
	setIndex(resp, out.Index)
	return out, nil
}

func (s *HTTPServer) jobVersions(resp http.ResponseWriter, req *http.Request,
	jobName string) (interface{}, error) {

	diffsStr := req.URL.Query().Get("diffs")
	var diffsBool bool
	if diffsStr != "" {
		var err error
		diffsBool, err = strconv.ParseBool(diffsStr)
		if err != nil {
			return nil, fmt.Errorf("Failed to parse value of %q (%v) as a bool: %v", "diffs", diffsStr, err)
		}
	}

	args := structs.JobVersionsRequest{
		JobID: jobName,
		Diffs: diffsBool,
	}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.JobVersionsResponse
	if err := s.agent.RPC("Job.GetJobVersions", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if len(out.Versions) == 0 {
		return nil, CodedError(404, "job versions not found")
	}

	return out, nil
}

func (s *HTTPServer) jobRevert(resp http.ResponseWriter, req *http.Request,
	jobName string) (interface{}, error) {

	if req.Method != "PUT" && req.Method != "POST" {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	var revertRequest structs.JobRevertRequest
	if err := decodeBody(req, &revertRequest); err != nil {
		return nil, CodedError(400, err.Error())
	}
	if revertRequest.JobID == "" {
		return nil, CodedError(400, "JobID must be specified")
	}
	if revertRequest.JobID != jobName {
		return nil, CodedError(400, "Job ID does not match")
	}

	s.parseWriteRequest(req, &revertRequest.WriteRequest)

	var out structs.JobRegisterResponse
	if err := s.agent.RPC("Job.Revert", &revertRequest, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	return out, nil
}

func (s *HTTPServer) jobStable(resp http.ResponseWriter, req *http.Request,
	jobName string) (interface{}, error) {

	if req.Method != "PUT" && req.Method != "POST" {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	var stableRequest structs.JobStabilityRequest
	if err := decodeBody(req, &stableRequest); err != nil {
		return nil, CodedError(400, err.Error())
	}
	if stableRequest.JobID == "" {
		return nil, CodedError(400, "JobID must be specified")
	}
	if stableRequest.JobID != jobName {
		return nil, CodedError(400, "Job ID does not match")
	}

	s.parseWriteRequest(req, &stableRequest.WriteRequest)

	var out structs.JobStabilityResponse
	if err := s.agent.RPC("Job.Stable", &stableRequest, &out); err != nil {
		return nil, err
	}

	setIndex(resp, out.Index)
	return out, nil
}

func (s *HTTPServer) jobSummaryRequest(resp http.ResponseWriter, req *http.Request, name string) (interface{}, error) {
	args := structs.JobSummaryRequest{
		JobID: name,
	}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.JobSummaryResponse
	if err := s.agent.RPC("Job.Summary", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.JobSummary == nil {
		return nil, CodedError(404, "job not found")
	}
	setIndex(resp, out.Index)
	return out.JobSummary, nil
}

func (s *HTTPServer) jobDispatchRequest(resp http.ResponseWriter, req *http.Request, name string) (interface{}, error) {
	if req.Method != "PUT" && req.Method != "POST" {
		return nil, CodedError(405, ErrInvalidMethod)
	}
	args := structs.JobDispatchRequest{}
	if err := decodeBody(req, &args); err != nil {
		return nil, CodedError(400, err.Error())
	}
	if args.JobID != "" && args.JobID != name {
		return nil, CodedError(400, "Job ID does not match")
	}
	if args.JobID == "" {
		args.JobID = name
	}

	s.parseWriteRequest(req, &args.WriteRequest)

	var out structs.JobDispatchResponse
	if err := s.agent.RPC("Job.Dispatch", &args, &out); err != nil {
		return nil, err
	}
	setIndex(resp, out.Index)
	return out, nil
}

// JobsParseRequest parses a hcl jobspec and returns a api.Job
func (s *HTTPServer) JobsParseRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != http.MethodPut && req.Method != http.MethodPost {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	args := &api.JobsParseRequest{}
	if err := decodeBody(req, &args); err != nil {
		return nil, CodedError(400, err.Error())
	}
	if args.JobHCL == "" {
		return nil, CodedError(400, "Job spec is empty")
	}

	jobfile := strings.NewReader(args.JobHCL)
	jobStruct, err := jobspec.Parse(jobfile)
	if err != nil {
		return nil, CodedError(400, err.Error())
	}

	if args.Canonicalize {
		jobStruct.Canonicalize()
	}
	return jobStruct, nil
}

func ApiJobToStructJob(job *api.Job) *structs.Job {
	job.Canonicalize()

	j := &structs.Job{
		Stop:        *job.Stop,
		Region:      *job.Region,
		Namespace:   *job.Namespace,
		ID:          *job.ID,
		ParentID:    *job.ParentID,
		Name:        *job.Name,
		Type:        *job.Type,
		Priority:    *job.Priority,
		AllAtOnce:   *job.AllAtOnce,
		Datacenters: job.Datacenters,
		Payload:     job.Payload,
		Meta:        job.Meta,
		VaultToken:  *job.VaultToken,
		Constraints: ApiConstraintsToStructs(job.Constraints),
		Affinities:  ApiAffinitiesToStructs(job.Affinities),
	}

	// COMPAT: Remove in 0.7.0. Update has been pushed into the task groups
	if job.Update != nil {
		j.Update = structs.UpdateStrategy{}

		if job.Update.Stagger != nil {
			j.Update.Stagger = *job.Update.Stagger
		}
		if job.Update.MaxParallel != nil {
			j.Update.MaxParallel = *job.Update.MaxParallel
		}
	}

	if l := len(job.Spreads); l != 0 {
		j.Spreads = make([]*structs.Spread, l)
		for i, apiSpread := range job.Spreads {
			j.Spreads[i] = ApiSpreadToStructs(apiSpread)
		}
	}

	if job.Periodic != nil {
		j.Periodic = &structs.PeriodicConfig{
			Enabled:         *job.Periodic.Enabled,
			SpecType:        *job.Periodic.SpecType,
			ProhibitOverlap: *job.Periodic.ProhibitOverlap,
			TimeZone:        *job.Periodic.TimeZone,
		}

		if job.Periodic.Spec != nil {
			j.Periodic.Spec = *job.Periodic.Spec
		}
	}

	if job.ParameterizedJob != nil {
		j.ParameterizedJob = &structs.ParameterizedJobConfig{
			Payload:      job.ParameterizedJob.Payload,
			MetaRequired: job.ParameterizedJob.MetaRequired,
			MetaOptional: job.ParameterizedJob.MetaOptional,
		}
	}

	if l := len(job.TaskGroups); l != 0 {
		j.TaskGroups = make([]*structs.TaskGroup, l)
		for i, taskGroup := range job.TaskGroups {
			tg := &structs.TaskGroup{}
			ApiTgToStructsTG(taskGroup, tg)
			j.TaskGroups[i] = tg
		}
	}

	return j
}

func ApiTgToStructsTG(taskGroup *api.TaskGroup, tg *structs.TaskGroup) {
	tg.Name = *taskGroup.Name
	tg.Count = *taskGroup.Count
	tg.Meta = taskGroup.Meta
	tg.Constraints = ApiConstraintsToStructs(taskGroup.Constraints)
	tg.Affinities = ApiAffinitiesToStructs(taskGroup.Affinities)

	tg.RestartPolicy = &structs.RestartPolicy{
		Attempts: *taskGroup.RestartPolicy.Attempts,
		Interval: *taskGroup.RestartPolicy.Interval,
		Delay:    *taskGroup.RestartPolicy.Delay,
		Mode:     *taskGroup.RestartPolicy.Mode,
	}

	if taskGroup.ReschedulePolicy != nil {
		tg.ReschedulePolicy = &structs.ReschedulePolicy{
			Attempts:      *taskGroup.ReschedulePolicy.Attempts,
			Interval:      *taskGroup.ReschedulePolicy.Interval,
			Delay:         *taskGroup.ReschedulePolicy.Delay,
			DelayFunction: *taskGroup.ReschedulePolicy.DelayFunction,
			MaxDelay:      *taskGroup.ReschedulePolicy.MaxDelay,
			Unlimited:     *taskGroup.ReschedulePolicy.Unlimited,
		}
	}

	if taskGroup.Migrate != nil {
		tg.Migrate = &structs.MigrateStrategy{
			MaxParallel:     *taskGroup.Migrate.MaxParallel,
			HealthCheck:     *taskGroup.Migrate.HealthCheck,
			MinHealthyTime:  *taskGroup.Migrate.MinHealthyTime,
			HealthyDeadline: *taskGroup.Migrate.HealthyDeadline,
		}
	}

	tg.EphemeralDisk = &structs.EphemeralDisk{
		Sticky:  *taskGroup.EphemeralDisk.Sticky,
		SizeMB:  *taskGroup.EphemeralDisk.SizeMB,
		Migrate: *taskGroup.EphemeralDisk.Migrate,
	}

	if l := len(taskGroup.Spreads); l != 0 {
		tg.Spreads = make([]*structs.Spread, l)
		for k, spread := range taskGroup.Spreads {
			tg.Spreads[k] = ApiSpreadToStructs(spread)
		}
	}

	if taskGroup.Update != nil {
		tg.Update = &structs.UpdateStrategy{
			Stagger:          *taskGroup.Update.Stagger,
			MaxParallel:      *taskGroup.Update.MaxParallel,
			HealthCheck:      *taskGroup.Update.HealthCheck,
			MinHealthyTime:   *taskGroup.Update.MinHealthyTime,
			HealthyDeadline:  *taskGroup.Update.HealthyDeadline,
			ProgressDeadline: *taskGroup.Update.ProgressDeadline,
			AutoRevert:       *taskGroup.Update.AutoRevert,
			Canary:           *taskGroup.Update.Canary,
		}
	}

	if l := len(taskGroup.Tasks); l != 0 {
		tg.Tasks = make([]*structs.Task, l)
		for l, task := range taskGroup.Tasks {
			t := &structs.Task{}
			ApiTaskToStructsTask(task, t)
			tg.Tasks[l] = t
		}
	}
}

// ApiTaskToStructsTask is a copy and type conversion between the API
// representation of a task from a struct representation of a task.
func ApiTaskToStructsTask(apiTask *api.Task, structsTask *structs.Task) {
	structsTask.Name = apiTask.Name
	structsTask.Driver = apiTask.Driver
	structsTask.User = apiTask.User
	structsTask.Leader = apiTask.Leader
	structsTask.Config = apiTask.Config
	structsTask.Env = apiTask.Env
	structsTask.Meta = apiTask.Meta
	structsTask.KillTimeout = *apiTask.KillTimeout
	structsTask.ShutdownDelay = apiTask.ShutdownDelay
	structsTask.KillSignal = apiTask.KillSignal
	structsTask.Constraints = ApiConstraintsToStructs(apiTask.Constraints)
	structsTask.Affinities = ApiAffinitiesToStructs(apiTask.Affinities)

	if l := len(apiTask.Services); l != 0 {
		structsTask.Services = make([]*structs.Service, l)
		for i, service := range apiTask.Services {
			structsTask.Services[i] = &structs.Service{
				Name:        service.Name,
				PortLabel:   service.PortLabel,
				Tags:        service.Tags,
				CanaryTags:  service.CanaryTags,
				AddressMode: service.AddressMode,
			}

			if l := len(service.Checks); l != 0 {
				structsTask.Services[i].Checks = make([]*structs.ServiceCheck, l)
				for j, check := range service.Checks {
					structsTask.Services[i].Checks[j] = &structs.ServiceCheck{
						Name:          check.Name,
						Type:          check.Type,
						Command:       check.Command,
						Args:          check.Args,
						Path:          check.Path,
						Protocol:      check.Protocol,
						PortLabel:     check.PortLabel,
						AddressMode:   check.AddressMode,
						Interval:      check.Interval,
						Timeout:       check.Timeout,
						InitialStatus: check.InitialStatus,
						TLSSkipVerify: check.TLSSkipVerify,
						Header:        check.Header,
						Method:        check.Method,
						GRPCService:   check.GRPCService,
						GRPCUseTLS:    check.GRPCUseTLS,
					}
					if check.CheckRestart != nil {
						structsTask.Services[i].Checks[j].CheckRestart = &structs.CheckRestart{
							Limit:          check.CheckRestart.Limit,
							Grace:          *check.CheckRestart.Grace,
							IgnoreWarnings: check.CheckRestart.IgnoreWarnings,
						}
					}
				}
			}
		}
	}

	structsTask.Resources = ApiResourcesToStructs(apiTask.Resources)

	structsTask.LogConfig = &structs.LogConfig{
		MaxFiles:      *apiTask.LogConfig.MaxFiles,
		MaxFileSizeMB: *apiTask.LogConfig.MaxFileSizeMB,
	}

	if l := len(apiTask.Artifacts); l != 0 {
		structsTask.Artifacts = make([]*structs.TaskArtifact, l)
		for k, ta := range apiTask.Artifacts {
			structsTask.Artifacts[k] = &structs.TaskArtifact{
				GetterSource:  *ta.GetterSource,
				GetterOptions: ta.GetterOptions,
				GetterMode:    *ta.GetterMode,
				RelativeDest:  *ta.RelativeDest,
			}
		}
	}

	if apiTask.Vault != nil {
		structsTask.Vault = &structs.Vault{
			Policies:     apiTask.Vault.Policies,
			Env:          *apiTask.Vault.Env,
			ChangeMode:   *apiTask.Vault.ChangeMode,
			ChangeSignal: *apiTask.Vault.ChangeSignal,
		}
	}

	if l := len(apiTask.Templates); l != 0 {
		structsTask.Templates = make([]*structs.Template, l)
		for i, template := range apiTask.Templates {
			structsTask.Templates[i] = &structs.Template{
				SourcePath:   *template.SourcePath,
				DestPath:     *template.DestPath,
				EmbeddedTmpl: *template.EmbeddedTmpl,
				ChangeMode:   *template.ChangeMode,
				ChangeSignal: *template.ChangeSignal,
				Splay:        *template.Splay,
				Perms:        *template.Perms,
				LeftDelim:    *template.LeftDelim,
				RightDelim:   *template.RightDelim,
				Envvars:      *template.Envvars,
				VaultGrace:   *template.VaultGrace,
			}
		}
	}

	if apiTask.DispatchPayload != nil {
		structsTask.DispatchPayload = &structs.DispatchPayloadConfig{
			File: apiTask.DispatchPayload.File,
		}
	}
}

func ApiResourcesToStructs(in *api.Resources) *structs.Resources {
	if in == nil {
		return nil
	}

	out := &structs.Resources{
		CPU:      *in.CPU,
		MemoryMB: *in.MemoryMB,
		IOPS:     *in.IOPS,
	}

	if l := len(in.Networks); l != 0 {
		out.Networks = make([]*structs.NetworkResource, l)
		for i, nw := range in.Networks {
			out.Networks[i] = &structs.NetworkResource{
				CIDR:  nw.CIDR,
				IP:    nw.IP,
				MBits: *nw.MBits,
			}

			if l := len(nw.DynamicPorts); l != 0 {
				out.Networks[i].DynamicPorts = make([]structs.Port, l)
				for j, dp := range nw.DynamicPorts {
					out.Networks[i].DynamicPorts[j] = structs.Port{
						Label: dp.Label,
						Value: dp.Value,
					}
				}
			}

			if l := len(nw.ReservedPorts); l != 0 {
				out.Networks[i].ReservedPorts = make([]structs.Port, l)
				for j, rp := range nw.ReservedPorts {
					out.Networks[i].ReservedPorts[j] = structs.Port{
						Label: rp.Label,
						Value: rp.Value,
					}
				}
			}
		}
	}

	if l := len(in.Devices); l != 0 {
		out.Devices = make([]*structs.RequestedDevice, l)
		for i, d := range in.Devices {
			out.Devices[i] = &structs.RequestedDevice{
				Name:        d.Name,
				Count:       *d.Count,
				Constraints: ApiConstraintsToStructs(d.Constraints),
				Affinities:  ApiAffinitiesToStructs(d.Affinities),
			}
		}
	}

	return out
}

func ApiConstraintsToStructs(in []*api.Constraint) []*structs.Constraint {
	if in == nil {
		return nil
	}

	out := make([]*structs.Constraint, len(in))
	for i, ac := range in {
		out[i] = ApiConstraintToStructs(ac)
	}

	return out
}

func ApiConstraintToStructs(in *api.Constraint) *structs.Constraint {
	if in == nil {
		return nil
	}

	return &structs.Constraint{
		LTarget: in.LTarget,
		RTarget: in.RTarget,
		Operand: in.Operand,
	}
}

func ApiAffinitiesToStructs(in []*api.Affinity) []*structs.Affinity {
	if in == nil {
		return nil
	}

	out := make([]*structs.Affinity, len(in))
	for i, ac := range in {
		out[i] = ApiAffinityToStructs(ac)
	}

	return out
}

func ApiAffinityToStructs(a1 *api.Affinity) *structs.Affinity {
	return &structs.Affinity{
		LTarget: a1.LTarget,
		Operand: a1.Operand,
		RTarget: a1.RTarget,
		Weight:  a1.Weight,
	}
}

func ApiSpreadToStructs(a1 *api.Spread) *structs.Spread {
	ret := &structs.Spread{}
	ret.Attribute = a1.Attribute
	ret.Weight = a1.Weight
	if a1.SpreadTarget != nil {
		ret.SpreadTarget = make([]*structs.SpreadTarget, len(a1.SpreadTarget))
		for i, st := range a1.SpreadTarget {
			ret.SpreadTarget[i] = &structs.SpreadTarget{
				Value:   st.Value,
				Percent: st.Percent,
			}
		}
	}
	return ret
}

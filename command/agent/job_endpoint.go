package agent

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/golang/snappy"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/api"
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
	default:
		return s.jobCRUD(resp, req, path)
	}
}

func (s *HTTPServer) jobForceEvaluate(resp http.ResponseWriter, req *http.Request,
	jobName string) (interface{}, error) {
	if req.Method != "PUT" && req.Method != "POST" {
		return nil, CodedError(405, ErrInvalidMethod)
	}
	args := structs.JobEvaluateRequest{
		JobID: jobName,
	}
	s.parseRegion(req, &args.Region)

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
	s.parseRegion(req, &args.Region)

	sJob := apiJobToStructJob(args.Job)
	planReq := structs.JobPlanRequest{
		Job:  sJob,
		Diff: args.Diff,
		WriteRequest: structs.WriteRequest{
			Region: args.WriteRequest.Region,
		},
	}
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

	job := apiJobToStructJob(validateRequest.Job)
	args := structs.JobValidateRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region: validateRequest.Region,
		},
	}
	s.parseRegion(req, &args.Region)

	var out structs.JobValidateResponse
	if err := s.agent.RPC("Job.Validate", &args, &out); err != nil {

		// Fall back to do local validation
		args.Job.Canonicalize()
		if vErr := args.Job.Validate(); vErr != nil {
			if merr, ok := vErr.(*multierror.Error); ok {
				for _, e := range merr.Errors {
					out.ValidationErrors = append(out.ValidationErrors, e.Error())
				}
			} else {
				out.ValidationErrors = append(out.ValidationErrors, vErr.Error())
			}
		}
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
	s.parseRegion(req, &args.Region)

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
	s.parseRegion(req, &args.Region)

	sJob := apiJobToStructJob(args.Job)

	regReq := structs.JobRegisterRequest{
		Job:            sJob,
		EnforceIndex:   args.EnforceIndex,
		JobModifyIndex: args.JobModifyIndex,
		WriteRequest: structs.WriteRequest{
			Region: args.WriteRequest.Region,
		},
	}
	var out structs.JobRegisterResponse
	if err := s.agent.RPC("Job.Register", &regReq, &out); err != nil {
		return nil, err
	}
	setIndex(resp, out.Index)
	return out, nil
}

func (s *HTTPServer) jobDelete(resp http.ResponseWriter, req *http.Request,
	jobName string) (interface{}, error) {
	args := structs.JobDeregisterRequest{
		JobID: jobName,
	}
	s.parseRegion(req, &args.Region)

	var out structs.JobDeregisterResponse
	if err := s.agent.RPC("Job.Deregister", &args, &out); err != nil {
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

	s.parseRegion(req, &args.Region)

	var out structs.JobDispatchResponse
	if err := s.agent.RPC("Job.Dispatch", &args, &out); err != nil {
		return nil, err
	}
	setIndex(resp, out.Index)
	return out, nil
}

func apiJobToStructJob(job *api.Job) *structs.Job {
	job.Canonicalize()

	j := &structs.Job{
		Region:            *job.Region,
		ID:                *job.ID,
		ParentID:          *job.ParentID,
		Name:              *job.Name,
		Type:              *job.Type,
		Priority:          *job.Priority,
		AllAtOnce:         *job.AllAtOnce,
		Datacenters:       job.Datacenters,
		Payload:           job.Payload,
		Meta:              job.Meta,
		VaultToken:        *job.VaultToken,
		Status:            *job.Status,
		StatusDescription: *job.StatusDescription,
		CreateIndex:       *job.CreateIndex,
		ModifyIndex:       *job.ModifyIndex,
		JobModifyIndex:    *job.JobModifyIndex,
	}

	j.Constraints = make([]*structs.Constraint, len(job.Constraints))
	for i, c := range job.Constraints {
		con := &structs.Constraint{}
		apiConstraintToStructs(c, con)
		j.Constraints[i] = con
	}
	if job.Update != nil {
		j.Update = structs.UpdateStrategy{
			Stagger:     job.Update.Stagger,
			MaxParallel: job.Update.MaxParallel,
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

	j.TaskGroups = make([]*structs.TaskGroup, len(job.TaskGroups))
	for i, taskGroup := range job.TaskGroups {
		tg := &structs.TaskGroup{}
		apiTgToStructsTG(taskGroup, tg)
		j.TaskGroups[i] = tg
	}

	return j
}

func apiTgToStructsTG(taskGroup *api.TaskGroup, tg *structs.TaskGroup) {
	tg.Name = *taskGroup.Name
	tg.Count = *taskGroup.Count
	tg.Meta = taskGroup.Meta
	tg.Constraints = make([]*structs.Constraint, len(taskGroup.Constraints))
	for k, constraint := range taskGroup.Constraints {
		c := &structs.Constraint{}
		apiConstraintToStructs(constraint, c)
		tg.Constraints[k] = c
	}
	tg.RestartPolicy = &structs.RestartPolicy{
		Attempts: *taskGroup.RestartPolicy.Attempts,
		Interval: *taskGroup.RestartPolicy.Interval,
		Delay:    *taskGroup.RestartPolicy.Delay,
		Mode:     *taskGroup.RestartPolicy.Mode,
	}
	tg.EphemeralDisk = &structs.EphemeralDisk{
		Sticky:  *taskGroup.EphemeralDisk.Sticky,
		SizeMB:  *taskGroup.EphemeralDisk.SizeMB,
		Migrate: *taskGroup.EphemeralDisk.Migrate,
	}
	tg.Meta = taskGroup.Meta
	tg.Tasks = make([]*structs.Task, len(taskGroup.Tasks))
	for l, task := range taskGroup.Tasks {
		t := &structs.Task{}
		apiTaskToStructsTask(task, t)
		tg.Tasks[l] = t
	}
}

func apiTaskToStructsTask(apiTask *api.Task, structsTask *structs.Task) {
	structsTask.Name = apiTask.Name
	structsTask.Driver = apiTask.Driver
	structsTask.User = apiTask.User
	structsTask.Leader = apiTask.Leader
	structsTask.Config = apiTask.Config
	structsTask.Constraints = make([]*structs.Constraint, len(apiTask.Constraints))
	for i, constraint := range apiTask.Constraints {
		c := &structs.Constraint{}
		apiConstraintToStructs(constraint, c)
		structsTask.Constraints[i] = c
	}
	structsTask.Env = apiTask.Env
	structsTask.Services = make([]*structs.Service, len(apiTask.Services))
	for i, service := range apiTask.Services {
		structsTask.Services[i] = &structs.Service{
			Name:      service.Name,
			PortLabel: service.PortLabel,
			Tags:      service.Tags,
		}
		structsTask.Services[i].Checks = make([]*structs.ServiceCheck, len(service.Checks))
		for j, check := range service.Checks {
			structsTask.Services[i].Checks[j] = &structs.ServiceCheck{
				Name:          check.Name,
				Type:          check.Type,
				Command:       check.Command,
				Args:          check.Args,
				Path:          check.Path,
				Protocol:      check.Protocol,
				PortLabel:     check.PortLabel,
				Interval:      check.Interval,
				Timeout:       check.Timeout,
				InitialStatus: check.InitialStatus,
			}
		}
	}
	structsTask.Resources = &structs.Resources{
		CPU:      *apiTask.Resources.CPU,
		MemoryMB: *apiTask.Resources.MemoryMB,
		IOPS:     *apiTask.Resources.IOPS,
	}
	structsTask.Resources.Networks = make([]*structs.NetworkResource, len(apiTask.Resources.Networks))
	for i, nw := range apiTask.Resources.Networks {
		structsTask.Resources.Networks[i] = &structs.NetworkResource{
			CIDR:  nw.CIDR,
			IP:    nw.IP,
			MBits: *nw.MBits,
		}
		structsTask.Resources.Networks[i].DynamicPorts = make([]structs.Port, len(nw.DynamicPorts))
		structsTask.Resources.Networks[i].ReservedPorts = make([]structs.Port, len(nw.ReservedPorts))
		for j, dp := range nw.DynamicPorts {
			structsTask.Resources.Networks[i].DynamicPorts[j] = structs.Port{
				Label: dp.Label,
				Value: dp.Value,
			}
		}
		for j, rp := range nw.ReservedPorts {
			structsTask.Resources.Networks[i].ReservedPorts[j] = structs.Port{
				Label: rp.Label,
				Value: rp.Value,
			}
		}
	}
	structsTask.Meta = apiTask.Meta
	structsTask.KillTimeout = *apiTask.KillTimeout
	structsTask.LogConfig = &structs.LogConfig{
		MaxFiles:      *apiTask.LogConfig.MaxFiles,
		MaxFileSizeMB: *apiTask.LogConfig.MaxFileSizeMB,
	}
	structsTask.Artifacts = make([]*structs.TaskArtifact, len(apiTask.Artifacts))
	for k, ta := range apiTask.Artifacts {
		structsTask.Artifacts[k] = &structs.TaskArtifact{
			GetterSource:  *ta.GetterSource,
			GetterOptions: ta.GetterOptions,
			RelativeDest:  *ta.RelativeDest,
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
	structsTask.Templates = make([]*structs.Template, len(apiTask.Templates))
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
		}
	}
	if apiTask.DispatchPayload != nil {
		structsTask.DispatchPayload = &structs.DispatchPayloadConfig{
			File: apiTask.DispatchPayload.File,
		}
	}
}

func apiConstraintToStructs(c1 *api.Constraint, c2 *structs.Constraint) {
	c2.LTarget = c1.LTarget
	c2.RTarget = c1.RTarget
	c2.Operand = c1.Operand
}

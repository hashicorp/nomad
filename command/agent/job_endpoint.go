// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/golang/snappy"
	"github.com/hashicorp/nomad/acl"
	api "github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/jobspec"
	"github.com/hashicorp/nomad/jobspec2"
	"github.com/hashicorp/nomad/nomad/structs"
)

// jobNotFoundErr is an error string which can be used as the return string
// alongside a 404 when a job is not found.
const jobNotFoundErr = "job not found"

func (s *HTTPServer) JobsRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	switch req.Method {
	case http.MethodGet:
		return s.jobListRequest(resp, req)
	case http.MethodPut, http.MethodPost:
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

	args.Fields = &structs.JobStubFields{}
	// Parse meta query param
	jobMeta, err := parseBool(req, "meta")
	if err != nil {
		return nil, err
	}
	if jobMeta != nil {
		args.Fields.Meta = *jobMeta
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
		jobID := strings.TrimSuffix(path, "/evaluate")
		return s.jobForceEvaluate(resp, req, jobID)
	case strings.HasSuffix(path, "/allocations"):
		jobID := strings.TrimSuffix(path, "/allocations")
		return s.jobAllocations(resp, req, jobID)
	case strings.HasSuffix(path, "/evaluations"):
		jobID := strings.TrimSuffix(path, "/evaluations")
		return s.jobEvaluations(resp, req, jobID)
	case strings.HasSuffix(path, "/periodic/force"):
		jobID := strings.TrimSuffix(path, "/periodic/force")
		return s.periodicForceRequest(resp, req, jobID)
	case strings.HasSuffix(path, "/plan"):
		jobID := strings.TrimSuffix(path, "/plan")
		return s.jobPlan(resp, req, jobID)
	case strings.HasSuffix(path, "/summary"):
		jobID := strings.TrimSuffix(path, "/summary")
		return s.jobSummaryRequest(resp, req, jobID)
	case strings.HasSuffix(path, "/dispatch"):
		jobID := strings.TrimSuffix(path, "/dispatch")
		return s.jobDispatchRequest(resp, req, jobID)
	case strings.HasSuffix(path, "/versions"):
		jobID := strings.TrimSuffix(path, "/versions")
		return s.jobVersions(resp, req, jobID)
	case strings.HasSuffix(path, "/revert"):
		jobID := strings.TrimSuffix(path, "/revert")
		return s.jobRevert(resp, req, jobID)
	case strings.HasSuffix(path, "/deployments"):
		jobID := strings.TrimSuffix(path, "/deployments")
		return s.jobDeployments(resp, req, jobID)
	case strings.HasSuffix(path, "/deployment"):
		jobID := strings.TrimSuffix(path, "/deployment")
		return s.jobLatestDeployment(resp, req, jobID)
	case strings.HasSuffix(path, "/stable"):
		jobID := strings.TrimSuffix(path, "/stable")
		return s.jobStable(resp, req, jobID)
	case strings.HasSuffix(path, "/scale"):
		jobID := strings.TrimSuffix(path, "/scale")
		return s.jobScale(resp, req, jobID)
	case strings.HasSuffix(path, "/services"):
		jobID := strings.TrimSuffix(path, "/services")
		return s.jobServiceRegistrations(resp, req, jobID)
	case strings.HasSuffix(path, "/submission"):
		jobID := strings.TrimSuffix(path, "/submission")
		return s.jobSubmissionCRUD(resp, req, jobID)
	default:
		return s.jobCRUD(resp, req, path)
	}
}

func (s *HTTPServer) jobForceEvaluate(resp http.ResponseWriter, req *http.Request, jobID string) (interface{}, error) {
	if req.Method != "PUT" && req.Method != "POST" {
		return nil, CodedError(405, ErrInvalidMethod)
	}
	var args structs.JobEvaluateRequest

	// TODO(preetha): remove in 0.9
	// COMPAT: For backwards compatibility allow using this endpoint without a payload
	if req.ContentLength == 0 {
		args = structs.JobEvaluateRequest{
			JobID: jobID,
		}
	} else {
		if err := decodeBody(req, &args); err != nil {
			return nil, CodedError(400, err.Error())
		}
		if args.JobID == "" {
			return nil, CodedError(400, "Job ID must be specified")
		}

		if jobID != "" && args.JobID != jobID {
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

	sJob, writeReq := s.apiJobAndRequestToStructs(args.Job, req, args.WriteRequest)
	planReq := structs.JobPlanRequest{
		Job:            sJob,
		Diff:           args.Diff,
		PolicyOverride: args.PolicyOverride,
		WriteRequest:   *writeReq,
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

func (s *HTTPServer) jobAllocations(resp http.ResponseWriter, req *http.Request, jobID string) (interface{}, error) {
	if req.Method != http.MethodGet {
		return nil, CodedError(405, ErrInvalidMethod)
	}
	allAllocs, _ := strconv.ParseBool(req.URL.Query().Get("all"))

	args := structs.JobSpecificRequest{
		JobID: jobID,
		All:   allAllocs,
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

func (s *HTTPServer) jobEvaluations(resp http.ResponseWriter, req *http.Request, jobID string) (interface{}, error) {
	if req.Method != http.MethodGet {
		return nil, CodedError(405, ErrInvalidMethod)
	}
	args := structs.JobSpecificRequest{
		JobID: jobID,
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

func (s *HTTPServer) jobDeployments(resp http.ResponseWriter, req *http.Request, jobID string) (interface{}, error) {
	if req.Method != http.MethodGet {
		return nil, CodedError(405, ErrInvalidMethod)
	}
	all, _ := strconv.ParseBool(req.URL.Query().Get("all"))
	args := structs.JobSpecificRequest{
		JobID: jobID,
		All:   all,
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

func (s *HTTPServer) jobLatestDeployment(resp http.ResponseWriter, req *http.Request, jobID string) (interface{}, error) {
	if req.Method != http.MethodGet {
		return nil, CodedError(405, ErrInvalidMethod)
	}
	args := structs.JobSpecificRequest{
		JobID: jobID,
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

func (s *HTTPServer) jobSubmissionCRUD(resp http.ResponseWriter, req *http.Request, jobID string) (*structs.JobSubmission, error) {
	version, err := strconv.ParseUint(req.URL.Query().Get("version"), 10, 64)
	if err != nil {
		return nil, CodedError(400, "Unable to parse job submission version parameter")
	}
	switch req.Method {
	case http.MethodGet:
		return s.jobSubmissionQuery(resp, req, jobID, version)
	default:
		return nil, CodedError(405, ErrInvalidMethod)
	}
}

func (s *HTTPServer) jobSubmissionQuery(resp http.ResponseWriter, req *http.Request, jobID string, version uint64) (*structs.JobSubmission, error) {
	args := structs.JobSubmissionRequest{
		JobID:   jobID,
		Version: version,
	}

	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.JobSubmissionResponse
	if err := s.agent.RPC("Job.GetJobSubmission", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.Submission == nil {
		return nil, CodedError(404, "job source not found")
	}

	return out.Submission, nil
}

func (s *HTTPServer) jobCRUD(resp http.ResponseWriter, req *http.Request, jobID string) (interface{}, error) {
	switch req.Method {
	case http.MethodGet:
		return s.jobQuery(resp, req, jobID)
	case http.MethodPut, http.MethodPost:
		return s.jobUpdate(resp, req, jobID)
	case http.MethodDelete:
		return s.jobDelete(resp, req, jobID)
	default:
		return nil, CodedError(405, ErrInvalidMethod)
	}
}

func (s *HTTPServer) jobQuery(resp http.ResponseWriter, req *http.Request, jobID string) (interface{}, error) {
	args := structs.JobSpecificRequest{
		JobID: jobID,
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

func (s *HTTPServer) jobUpdate(resp http.ResponseWriter, req *http.Request, jobID string) (interface{}, error) {
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
	if jobID != "" && *args.Job.ID != jobID {
		return nil, CodedError(400, "Job ID does not match name")
	}

	// GH-8481. Jobs of type system can only have a count of 1 and therefore do
	// not support scaling. Even though this returns an error on the first
	// occurrence, the error is generic but detailed enough that an operator
	// can fix the problem across multiple task groups.
	if args.Job.Type != nil && *args.Job.Type == api.JobTypeSystem {
		for _, tg := range args.Job.TaskGroups {
			if tg.Scaling != nil {
				return nil, CodedError(400, "Task groups with job type system do not support scaling blocks")
			}
		}
	}

	// Validate the evaluation priority if the user supplied a non-default
	// value. It's more efficient to do it here, within the agent rather than
	// sending a bad request for the server to reject.
	if args.EvalPriority != 0 {
		if err := validateEvalPriorityOpt(args.EvalPriority); err != nil {
			return nil, err
		}
	}

	sJob, writeReq := s.apiJobAndRequestToStructs(args.Job, req, args.WriteRequest)
	submission := apiJobSubmissionToStructs(args.Submission)

	regReq := structs.JobRegisterRequest{
		Job:        sJob,
		Submission: submission,

		EnforceIndex:   args.EnforceIndex,
		JobModifyIndex: args.JobModifyIndex,
		PolicyOverride: args.PolicyOverride,
		PreserveCounts: args.PreserveCounts,
		EvalPriority:   args.EvalPriority,
		WriteRequest:   *writeReq,
	}

	var out structs.JobRegisterResponse
	if err := s.agent.RPC("Job.Register", &regReq, &out); err != nil {
		return nil, err
	}
	setIndex(resp, out.Index)
	return out, nil
}

func (s *HTTPServer) jobDelete(resp http.ResponseWriter, req *http.Request, jobID string) (interface{}, error) {

	args := structs.JobDeregisterRequest{
		JobID: jobID,
	}

	// Identify the purge query param and parse.
	purgeStr := req.URL.Query().Get("purge")
	var purgeBool bool
	if purgeStr != "" {
		var err error
		purgeBool, err = strconv.ParseBool(purgeStr)
		if err != nil {
			return nil, fmt.Errorf("Failed to parse value of %q (%v) as a bool: %v", "purge", purgeStr, err)
		}
	}
	args.Purge = purgeBool

	// Identify the global query param and parse.
	globalStr := req.URL.Query().Get("global")
	var globalBool bool
	if globalStr != "" {
		var err error
		globalBool, err = strconv.ParseBool(globalStr)
		if err != nil {
			return nil, fmt.Errorf("Failed to parse value of %q (%v) as a bool: %v", "global", globalStr, err)
		}
	}
	args.Global = globalBool

	// Parse the eval priority from the request URL query if present.
	evalPriority, err := parseInt(req, "eval_priority")
	if err != nil {
		return nil, err
	}

	// Identify the no_shutdown_delay query param and parse.
	noShutdownDelayStr := req.URL.Query().Get("no_shutdown_delay")
	var noShutdownDelay bool
	if noShutdownDelayStr != "" {
		var err error
		noShutdownDelay, err = strconv.ParseBool(noShutdownDelayStr)
		if err != nil {
			return nil, fmt.Errorf("Failed to parse value of %qq (%v) as a bool: %v", "no_shutdown_delay", noShutdownDelayStr, err)
		}
	}
	args.NoShutdownDelay = noShutdownDelay

	// Validate the evaluation priority if the user supplied a non-default
	// value. It's more efficient to do it here, within the agent rather than
	// sending a bad request for the server to reject.
	if evalPriority != nil && *evalPriority > 0 {
		if err := validateEvalPriorityOpt(*evalPriority); err != nil {
			return nil, err
		}
		args.EvalPriority = *evalPriority
	}

	s.parseWriteRequest(req, &args.WriteRequest)

	var out structs.JobDeregisterResponse
	if err := s.agent.RPC("Job.Deregister", &args, &out); err != nil {
		return nil, err
	}
	setIndex(resp, out.Index)
	return out, nil
}

func (s *HTTPServer) jobScale(resp http.ResponseWriter, req *http.Request, jobID string) (interface{}, error) {

	switch req.Method {
	case http.MethodGet:
		return s.jobScaleStatus(resp, req, jobID)
	case http.MethodPut, http.MethodPost:
		return s.jobScaleAction(resp, req, jobID)
	default:
		return nil, CodedError(405, ErrInvalidMethod)
	}
}

func (s *HTTPServer) jobScaleStatus(resp http.ResponseWriter, req *http.Request, jobID string) (interface{}, error) {

	args := structs.JobScaleStatusRequest{
		JobID: jobID,
	}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.JobScaleStatusResponse
	if err := s.agent.RPC("Job.ScaleStatus", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.JobScaleStatus == nil {
		return nil, CodedError(404, "job not found")
	}

	return out.JobScaleStatus, nil
}

func (s *HTTPServer) jobScaleAction(resp http.ResponseWriter, req *http.Request, jobID string) (interface{}, error) {

	if req.Method != "PUT" && req.Method != "POST" {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	var args api.ScalingRequest
	if err := decodeBody(req, &args); err != nil {
		return nil, CodedError(400, err.Error())
	}

	targetJob := args.Target[structs.ScalingTargetJob]
	if targetJob != "" && targetJob != jobID {
		return nil, CodedError(400, "job ID in payload did not match URL")
	}

	scaleReq := structs.JobScaleRequest{
		JobID:          jobID,
		Target:         args.Target,
		Count:          args.Count,
		PolicyOverride: args.PolicyOverride,
		Message:        args.Message,
		Error:          args.Error,
		Meta:           args.Meta,
	}
	// parseWriteRequest overrides Namespace, Region and AuthToken
	// based on values from the original http request
	s.parseWriteRequest(req, &scaleReq.WriteRequest)

	var out structs.JobRegisterResponse
	if err := s.agent.RPC("Job.Scale", &scaleReq, &out); err != nil {
		return nil, err
	}
	setIndex(resp, out.Index)
	return out, nil
}

func (s *HTTPServer) jobVersions(resp http.ResponseWriter, req *http.Request, jobID string) (interface{}, error) {

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
		JobID: jobID,
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

func (s *HTTPServer) jobRevert(resp http.ResponseWriter, req *http.Request, jobID string) (interface{}, error) {

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
	if revertRequest.JobID != jobID {
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

func (s *HTTPServer) jobStable(resp http.ResponseWriter, req *http.Request, jobID string) (interface{}, error) {

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
	if stableRequest.JobID != jobID {
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

func (s *HTTPServer) jobSummaryRequest(resp http.ResponseWriter, req *http.Request, jobID string) (interface{}, error) {
	args := structs.JobSummaryRequest{
		JobID: jobID,
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

func (s *HTTPServer) jobDispatchRequest(resp http.ResponseWriter, req *http.Request, jobID string) (interface{}, error) {
	if req.Method != "PUT" && req.Method != "POST" {
		return nil, CodedError(405, ErrInvalidMethod)
	}
	args := structs.JobDispatchRequest{}
	if err := decodeBody(req, &args); err != nil {
		return nil, CodedError(400, err.Error())
	}
	if args.JobID != "" && args.JobID != jobID {
		return nil, CodedError(400, "Job ID does not match")
	}
	if args.JobID == "" {
		args.JobID = jobID
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

	var namespace string
	parseNamespace(req, &namespace)

	aclObj, err := s.ResolveToken(req)
	if err != nil {
		return nil, err
	}

	// Check job parse permissions
	if aclObj != nil {
		hasParseJob := aclObj.AllowNsOp(namespace, acl.NamespaceCapabilityParseJob)
		hasSubmitJob := aclObj.AllowNsOp(namespace, acl.NamespaceCapabilitySubmitJob)

		allowed := hasParseJob || hasSubmitJob
		if !allowed {
			return nil, structs.ErrPermissionDenied
		}
	}

	args := &api.JobsParseRequest{}
	if err := decodeBody(req, &args); err != nil {
		return nil, CodedError(400, err.Error())
	}
	if args.JobHCL == "" {
		return nil, CodedError(400, "Job spec is empty")
	}

	var jobStruct *api.Job
	if args.HCLv1 {
		jobStruct, err = jobspec.Parse(strings.NewReader(args.JobHCL))
	} else {
		jobStruct, err = jobspec2.ParseWithConfig(&jobspec2.ParseConfig{
			Path:       "input.hcl",
			Body:       []byte(args.JobHCL),
			AllowFS:    false,
			VarContent: args.Variables,
		})
		if err != nil {
			return nil, CodedError(400, fmt.Sprintf("Failed to parse job: %v", err))
		}
	}
	if err != nil {
		return nil, CodedError(400, err.Error())
	}

	if args.Canonicalize {
		jobStruct.Canonicalize()
	}
	return jobStruct, nil
}

// jobServiceRegistrations returns a list of all service registrations assigned
// to the job identifier. It is callable via the
// /v1/job/:jobID/services HTTP API and uses the
// structs.JobServiceRegistrationsRPCMethod RPC method.
func (s *HTTPServer) jobServiceRegistrations(resp http.ResponseWriter, req *http.Request, jobID string) (interface{}, error) {

	// The endpoint only supports GET requests.
	if req.Method != http.MethodGet {
		return nil, CodedError(http.StatusMethodNotAllowed, ErrInvalidMethod)
	}

	// Set up the request args and parse this to ensure the query options are
	// set.
	args := structs.JobServiceRegistrationsRequest{JobID: jobID}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	// Perform the RPC request.
	var reply structs.JobServiceRegistrationsResponse
	if err := s.agent.RPC(structs.JobServiceRegistrationsRPCMethod, &args, &reply); err != nil {
		return nil, err
	}

	setMeta(resp, &reply.QueryMeta)

	if reply.Services == nil {
		return nil, CodedError(http.StatusNotFound, jobNotFoundErr)
	}
	return reply.Services, nil
}

func apiJobSubmissionToStructs(submission *api.JobSubmission) *structs.JobSubmission {
	if submission == nil {
		return nil
	}
	return &structs.JobSubmission{
		Source:        submission.Source,
		Format:        submission.Format,
		VariableFlags: submission.VariableFlags,
		Variables:     submission.Variables,
	}
}

// apiJobAndRequestToStructs parses the query params from the incoming
// request and converts to a structs.Job and WriteRequest with the
func (s *HTTPServer) apiJobAndRequestToStructs(job *api.Job, req *http.Request, apiReq api.WriteRequest) (*structs.Job, *structs.WriteRequest) {

	// parseWriteRequest gets the Namespace, Region, and AuthToken from
	// the original HTTP request's query params and headers and overrides
	// those values set in the request body
	writeReq := &structs.WriteRequest{
		Namespace: apiReq.Namespace,
		Region:    apiReq.Region,
		AuthToken: apiReq.SecretID,
	}

	s.parseToken(req, &writeReq.AuthToken)

	queryRegion := req.URL.Query().Get("region")
	requestRegion, jobRegion := regionForJob(
		job, queryRegion, writeReq.Region, s.agent.GetConfig().Region,
	)

	sJob := ApiJobToStructJob(job)
	sJob.Region = jobRegion
	writeReq.Region = requestRegion

	queryNamespace := req.URL.Query().Get("namespace")
	namespace := namespaceForJob(job.Namespace, queryNamespace, writeReq.Namespace)
	sJob.Namespace = namespace
	writeReq.Namespace = namespace

	return sJob, writeReq
}

func regionForJob(job *api.Job, queryRegion, apiRegion, agentRegion string) (string, string) {
	var requestRegion string
	var jobRegion string

	// Region in query param (-region flag) takes precedence.
	if queryRegion != "" {
		requestRegion = queryRegion
		jobRegion = queryRegion
	}

	// Next the request body...
	if apiRegion != "" {
		requestRegion = apiRegion
		jobRegion = apiRegion
	}

	// If no query param was passed, we forward to the job's region
	// if one is available
	if requestRegion == "" && job.Region != nil {
		requestRegion = *job.Region
		jobRegion = *job.Region
	}

	// otherwise we default to the region of this node
	if requestRegion == "" || requestRegion == api.GlobalRegion {
		requestRegion = agentRegion
		jobRegion = agentRegion
	}

	// Multiregion jobs have their job region set to the global region,
	// and enforce that we forward to a region where they will be deployed
	if job.Multiregion != nil {
		jobRegion = api.GlobalRegion

		// multiregion jobs with 0 regions won't pass validation,
		// but this protects us from NPE
		if len(job.Multiregion.Regions) > 0 {
			found := false
			for _, region := range job.Multiregion.Regions {
				if region.Name == requestRegion {
					found = true
				}
			}
			if !found {
				requestRegion = job.Multiregion.Regions[0].Name
			}
		}
	}

	return requestRegion, jobRegion
}

func namespaceForJob(jobNamespace *string, queryNamespace, apiNamespace string) string {

	// Namespace in query param (-namespace flag) takes precedence.
	if queryNamespace != "" {
		return queryNamespace
	}

	// Next the request body...
	if apiNamespace != "" {
		return apiNamespace
	}

	if jobNamespace != nil && *jobNamespace != "" {
		return *jobNamespace
	}

	return structs.DefaultNamespace
}

func ApiJobToStructJob(job *api.Job) *structs.Job {
	job.Canonicalize()

	j := &structs.Job{
		Stop:           *job.Stop,
		Region:         *job.Region,
		Namespace:      *job.Namespace,
		ID:             *job.ID,
		Name:           *job.Name,
		Type:           *job.Type,
		Priority:       *job.Priority,
		AllAtOnce:      *job.AllAtOnce,
		Datacenters:    job.Datacenters,
		NodePool:       *job.NodePool,
		Payload:        job.Payload,
		Meta:           job.Meta,
		ConsulToken:    *job.ConsulToken,
		VaultToken:     *job.VaultToken,
		VaultNamespace: *job.VaultNamespace,
		Constraints:    ApiConstraintsToStructs(job.Constraints),
		Affinities:     ApiAffinitiesToStructs(job.Affinities),
	}

	// Update has been pushed into the task groups. stagger and max_parallel are
	// preserved at the job level, but all other values are discarded. The job.Update
	// api value is merged into TaskGroups already in api.Canonicalize
	if job.Update != nil && job.Update.MaxParallel != nil && *job.Update.MaxParallel > 0 {
		j.Update = structs.UpdateStrategy{}

		if job.Update.Stagger != nil {
			j.Update.Stagger = *job.Update.Stagger
		}
		if job.Update.MaxParallel != nil {
			j.Update.MaxParallel = *job.Update.MaxParallel
		}
	}

	if len(job.Spreads) > 0 {
		j.Spreads = []*structs.Spread{}
		for _, apiSpread := range job.Spreads {
			j.Spreads = append(j.Spreads, ApiSpreadToStructs(apiSpread))
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

		if job.Periodic.Specs != nil {
			j.Periodic.Specs = job.Periodic.Specs
		}
	}

	if job.ParameterizedJob != nil {
		j.ParameterizedJob = &structs.ParameterizedJobConfig{
			Payload:      job.ParameterizedJob.Payload,
			MetaRequired: job.ParameterizedJob.MetaRequired,
			MetaOptional: job.ParameterizedJob.MetaOptional,
		}
	}

	if job.Multiregion != nil {
		j.Multiregion = &structs.Multiregion{}
		j.Multiregion.Strategy = &structs.MultiregionStrategy{
			MaxParallel: *job.Multiregion.Strategy.MaxParallel,
			OnFailure:   *job.Multiregion.Strategy.OnFailure,
		}
		j.Multiregion.Regions = []*structs.MultiregionRegion{}
		for _, region := range job.Multiregion.Regions {
			r := &structs.MultiregionRegion{}
			r.Name = region.Name
			r.Count = *region.Count
			r.Datacenters = region.Datacenters
			r.NodePool = region.NodePool
			r.Meta = region.Meta
			j.Multiregion.Regions = append(j.Multiregion.Regions, r)
		}
	}

	if len(job.TaskGroups) > 0 {
		j.TaskGroups = []*structs.TaskGroup{}
		for _, taskGroup := range job.TaskGroups {
			tg := &structs.TaskGroup{}
			ApiTgToStructsTG(j, taskGroup, tg)
			j.TaskGroups = append(j.TaskGroups, tg)
		}
	}

	return j
}

func ApiTgToStructsTG(job *structs.Job, taskGroup *api.TaskGroup, tg *structs.TaskGroup) {
	tg.Name = *taskGroup.Name
	tg.Count = *taskGroup.Count
	tg.Meta = taskGroup.Meta
	tg.Constraints = ApiConstraintsToStructs(taskGroup.Constraints)
	tg.Affinities = ApiAffinitiesToStructs(taskGroup.Affinities)
	tg.Networks = ApiNetworkResourceToStructs(taskGroup.Networks)
	tg.Services = ApiServicesToStructs(taskGroup.Services, true)
	tg.Consul = apiConsulToStructs(taskGroup.Consul)

	tg.RestartPolicy = &structs.RestartPolicy{
		Attempts:        *taskGroup.RestartPolicy.Attempts,
		Interval:        *taskGroup.RestartPolicy.Interval,
		Delay:           *taskGroup.RestartPolicy.Delay,
		Mode:            *taskGroup.RestartPolicy.Mode,
		RenderTemplates: *taskGroup.RestartPolicy.RenderTemplates,
	}

	if taskGroup.ShutdownDelay != nil {
		tg.ShutdownDelay = taskGroup.ShutdownDelay
	}

	if taskGroup.StopAfterClientDisconnect != nil {
		tg.StopAfterClientDisconnect = taskGroup.StopAfterClientDisconnect
	}

	if taskGroup.MaxClientDisconnect != nil {
		tg.MaxClientDisconnect = taskGroup.MaxClientDisconnect
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

	if taskGroup.Scaling != nil {
		tg.Scaling = ApiScalingPolicyToStructs(tg.Count, taskGroup.Scaling).TargetTaskGroup(job, tg)
	}

	tg.EphemeralDisk = &structs.EphemeralDisk{
		Sticky:  *taskGroup.EphemeralDisk.Sticky,
		SizeMB:  *taskGroup.EphemeralDisk.SizeMB,
		Migrate: *taskGroup.EphemeralDisk.Migrate,
	}

	if len(taskGroup.Spreads) > 0 {
		tg.Spreads = []*structs.Spread{}
		for _, spread := range taskGroup.Spreads {
			tg.Spreads = append(tg.Spreads, ApiSpreadToStructs(spread))
		}
	}

	if len(taskGroup.Volumes) > 0 {
		tg.Volumes = map[string]*structs.VolumeRequest{}
		for k, v := range taskGroup.Volumes {
			if v == nil || (v.Type != structs.VolumeTypeHost && v.Type != structs.VolumeTypeCSI) {
				// Ignore volumes we don't understand in this iteration currently.
				// - This is because we don't currently have a way to return errors here.
				continue
			}

			vol := &structs.VolumeRequest{
				Name:           v.Name,
				Type:           v.Type,
				ReadOnly:       v.ReadOnly,
				Source:         v.Source,
				AttachmentMode: structs.CSIVolumeAttachmentMode(v.AttachmentMode),
				AccessMode:     structs.CSIVolumeAccessMode(v.AccessMode),
				PerAlloc:       v.PerAlloc,
			}

			if v.MountOptions != nil {
				vol.MountOptions = &structs.CSIMountOptions{
					FSType:     v.MountOptions.FSType,
					MountFlags: v.MountOptions.MountFlags,
				}
			}

			tg.Volumes[k] = vol
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
			Canary:           *taskGroup.Update.Canary,
		}

		// boolPtr fields may be nil, others will have pointers to default values via Canonicalize
		if taskGroup.Update.AutoRevert != nil {
			tg.Update.AutoRevert = *taskGroup.Update.AutoRevert
		}

		if taskGroup.Update.AutoPromote != nil {
			tg.Update.AutoPromote = *taskGroup.Update.AutoPromote
		}
	}

	if len(taskGroup.Tasks) > 0 {
		tg.Tasks = []*structs.Task{}
		for _, task := range taskGroup.Tasks {
			t := &structs.Task{}
			ApiTaskToStructsTask(job, tg, task, t)

			// Set the tasks vault namespace from Job if it was not
			// specified by the task or group
			if t.Vault != nil && t.Vault.Namespace == "" && job.VaultNamespace != "" {
				t.Vault.Namespace = job.VaultNamespace
			}
			tg.Tasks = append(tg.Tasks, t)
		}
	}
}

// ApiTaskToStructsTask is a copy and type conversion between the API
// representation of a task from a struct representation of a task.
func ApiTaskToStructsTask(job *structs.Job, group *structs.TaskGroup,
	apiTask *api.Task, structsTask *structs.Task) {

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
	structsTask.Kind = structs.TaskKind(apiTask.Kind)
	structsTask.Constraints = ApiConstraintsToStructs(apiTask.Constraints)
	structsTask.Affinities = ApiAffinitiesToStructs(apiTask.Affinities)
	structsTask.CSIPluginConfig = ApiCSIPluginConfigToStructsCSIPluginConfig(apiTask.CSIPluginConfig)

	// Nomad 1.5 CLIs and JSON jobs may set the default identity parameters in
	// the Task.Identity field, so if it is non-nil use it.
	if id := apiTask.Identity; id != nil {
		structsTask.Identity = &structs.WorkloadIdentity{
			Name:     id.Name,
			Audience: slices.Clone(id.Audience),
			Env:      id.Env,
			File:     id.File,
		}
	}

	if ids := apiTask.Identities; len(ids) > 0 {
		structsTask.Identities = make([]*structs.WorkloadIdentity, len(ids))
		for i, id := range ids {
			if id == nil {
				continue
			}

			structsTask.Identities[i] = &structs.WorkloadIdentity{
				Name:     id.Name,
				Audience: slices.Clone(id.Audience),
				Env:      id.Env,
				File:     id.File,
			}

		}
	}

	if apiTask.RestartPolicy != nil {
		structsTask.RestartPolicy = &structs.RestartPolicy{
			Attempts:        *apiTask.RestartPolicy.Attempts,
			Interval:        *apiTask.RestartPolicy.Interval,
			Delay:           *apiTask.RestartPolicy.Delay,
			Mode:            *apiTask.RestartPolicy.Mode,
			RenderTemplates: *apiTask.RestartPolicy.RenderTemplates,
		}
	}

	if len(apiTask.VolumeMounts) > 0 {
		structsTask.VolumeMounts = []*structs.VolumeMount{}
		for _, mount := range apiTask.VolumeMounts {
			if mount != nil && mount.Volume != nil {
				structsTask.VolumeMounts = append(structsTask.VolumeMounts,
					&structs.VolumeMount{
						Volume:          *mount.Volume,
						Destination:     *mount.Destination,
						ReadOnly:        *mount.ReadOnly,
						PropagationMode: *mount.PropagationMode,
					})
			}
		}
	}

	if len(apiTask.ScalingPolicies) > 0 {
		structsTask.ScalingPolicies = []*structs.ScalingPolicy{}
		for _, policy := range apiTask.ScalingPolicies {
			structsTask.ScalingPolicies = append(
				structsTask.ScalingPolicies,
				ApiScalingPolicyToStructs(0, policy).TargetTask(job, group, structsTask))
		}
	}

	structsTask.Services = ApiServicesToStructs(apiTask.Services, false)

	structsTask.Resources = ApiResourcesToStructs(apiTask.Resources)

	structsTask.LogConfig = apiLogConfigToStructs(apiTask.LogConfig)

	if len(apiTask.Artifacts) > 0 {
		structsTask.Artifacts = []*structs.TaskArtifact{}
		for _, ta := range apiTask.Artifacts {
			structsTask.Artifacts = append(structsTask.Artifacts,
				&structs.TaskArtifact{
					GetterSource:  *ta.GetterSource,
					GetterOptions: maps.Clone(ta.GetterOptions),
					GetterHeaders: maps.Clone(ta.GetterHeaders),
					GetterMode:    *ta.GetterMode,
					RelativeDest:  *ta.RelativeDest,
				})
		}
	}

	if apiTask.Vault != nil {
		structsTask.Vault = &structs.Vault{
			Role:         apiTask.Vault.Role,
			Policies:     apiTask.Vault.Policies,
			Namespace:    *apiTask.Vault.Namespace,
			Env:          *apiTask.Vault.Env,
			DisableFile:  *apiTask.Vault.DisableFile,
			ChangeMode:   *apiTask.Vault.ChangeMode,
			ChangeSignal: *apiTask.Vault.ChangeSignal,
		}
	}

	if len(apiTask.Templates) > 0 {
		structsTask.Templates = []*structs.Template{}
		for _, template := range apiTask.Templates {
			structsTask.Templates = append(structsTask.Templates,
				&structs.Template{
					SourcePath:    *template.SourcePath,
					DestPath:      *template.DestPath,
					EmbeddedTmpl:  *template.EmbeddedTmpl,
					ChangeMode:    *template.ChangeMode,
					ChangeSignal:  *template.ChangeSignal,
					ChangeScript:  apiChangeScriptToStructsChangeScript(template.ChangeScript),
					Splay:         *template.Splay,
					Perms:         *template.Perms,
					Uid:           template.Uid,
					Gid:           template.Gid,
					LeftDelim:     *template.LeftDelim,
					RightDelim:    *template.RightDelim,
					Envvars:       *template.Envvars,
					VaultGrace:    *template.VaultGrace,
					Wait:          apiWaitConfigToStructsWaitConfig(template.Wait),
					ErrMissingKey: *template.ErrMissingKey,
				})
		}
	}

	if apiTask.DispatchPayload != nil {
		structsTask.DispatchPayload = &structs.DispatchPayloadConfig{
			File: apiTask.DispatchPayload.File,
		}
	}

	if apiTask.Lifecycle != nil {
		structsTask.Lifecycle = &structs.TaskLifecycleConfig{
			Hook:    apiTask.Lifecycle.Hook,
			Sidecar: apiTask.Lifecycle.Sidecar,
		}
	}
}

// apiWaitConfigToStructsWaitConfig is a copy and type conversion between the API
// representation of a WaitConfig from a struct representation of a WaitConfig.
func apiWaitConfigToStructsWaitConfig(waitConfig *api.WaitConfig) *structs.WaitConfig {
	if waitConfig == nil {
		return nil
	}

	return &structs.WaitConfig{
		Min: waitConfig.Min,
		Max: waitConfig.Max,
	}
}

func apiChangeScriptToStructsChangeScript(changeScript *api.ChangeScript) *structs.ChangeScript {
	if changeScript == nil {
		return nil
	}

	return &structs.ChangeScript{
		Command:     *changeScript.Command,
		Args:        changeScript.Args,
		Timeout:     *changeScript.Timeout,
		FailOnError: *changeScript.FailOnError,
	}
}

func ApiCSIPluginConfigToStructsCSIPluginConfig(apiConfig *api.TaskCSIPluginConfig) *structs.TaskCSIPluginConfig {
	if apiConfig == nil {
		return nil
	}

	sc := &structs.TaskCSIPluginConfig{}
	sc.ID = apiConfig.ID
	sc.Type = structs.CSIPluginType(apiConfig.Type)
	sc.MountDir = apiConfig.MountDir
	sc.StagePublishBaseDir = apiConfig.StagePublishBaseDir
	sc.HealthTimeout = apiConfig.HealthTimeout
	return sc
}

func ApiResourcesToStructs(in *api.Resources) *structs.Resources {
	if in == nil {
		return nil
	}

	out := &structs.Resources{
		CPU:      *in.CPU,
		MemoryMB: *in.MemoryMB,
	}

	if in.Cores != nil {
		out.Cores = *in.Cores
	}

	if in.MemoryMaxMB != nil {
		out.MemoryMaxMB = *in.MemoryMaxMB
	}

	// COMPAT(0.10): Only being used to issue warnings
	if in.IOPS != nil {
		out.IOPS = *in.IOPS
	}

	if len(in.Networks) != 0 {
		out.Networks = ApiNetworkResourceToStructs(in.Networks)
	}

	if len(in.Devices) > 0 {
		out.Devices = []*structs.RequestedDevice{}
		for _, d := range in.Devices {
			out.Devices = append(out.Devices, &structs.RequestedDevice{
				Name:        d.Name,
				Count:       *d.Count,
				Constraints: ApiConstraintsToStructs(d.Constraints),
				Affinities:  ApiAffinitiesToStructs(d.Affinities),
			})
		}
	}

	return out
}

func ApiNetworkResourceToStructs(in []*api.NetworkResource) []*structs.NetworkResource {
	var out []*structs.NetworkResource
	if len(in) == 0 {
		return out
	}
	out = make([]*structs.NetworkResource, len(in))
	for i, nw := range in {
		out[i] = &structs.NetworkResource{
			Mode:     nw.Mode,
			CIDR:     nw.CIDR,
			IP:       nw.IP,
			Hostname: nw.Hostname,
			MBits:    nw.Megabits(),
		}

		if nw.DNS != nil {
			out[i].DNS = &structs.DNSConfig{
				Servers:  nw.DNS.Servers,
				Searches: nw.DNS.Searches,
				Options:  nw.DNS.Options,
			}
		}

		if l := len(nw.DynamicPorts); l != 0 {
			out[i].DynamicPorts = make([]structs.Port, l)
			for j, dp := range nw.DynamicPorts {
				out[i].DynamicPorts[j] = ApiPortToStructs(dp)
			}
		}

		if l := len(nw.ReservedPorts); l != 0 {
			out[i].ReservedPorts = make([]structs.Port, l)
			for j, rp := range nw.ReservedPorts {
				out[i].ReservedPorts[j] = ApiPortToStructs(rp)
			}
		}
	}

	return out
}

func ApiPortToStructs(in api.Port) structs.Port {
	return structs.Port{
		Label:       in.Label,
		Value:       in.Value,
		To:          in.To,
		HostNetwork: in.HostNetwork,
	}
}

func ApiServicesToStructs(in []*api.Service, group bool) []*structs.Service {
	if len(in) == 0 {
		return nil
	}

	out := make([]*structs.Service, len(in))
	for i, s := range in {
		out[i] = &structs.Service{
			Name:              s.Name,
			PortLabel:         s.PortLabel,
			TaskName:          s.TaskName,
			Tags:              s.Tags,
			CanaryTags:        s.CanaryTags,
			EnableTagOverride: s.EnableTagOverride,
			AddressMode:       s.AddressMode,
			Address:           s.Address,
			Meta:              maps.Clone(s.Meta),
			CanaryMeta:        maps.Clone(s.CanaryMeta),
			TaggedAddresses:   maps.Clone(s.TaggedAddresses),
			OnUpdate:          s.OnUpdate,
			Provider:          s.Provider,
		}

		if l := len(s.Checks); l != 0 {
			out[i].Checks = make([]*structs.ServiceCheck, l)
			for j, check := range s.Checks {
				onUpdate := s.OnUpdate // Inherit from service as default
				if check.OnUpdate != "" {
					onUpdate = check.OnUpdate
				}
				out[i].Checks[j] = &structs.ServiceCheck{
					Name:                   check.Name,
					Type:                   check.Type,
					Command:                check.Command,
					Args:                   check.Args,
					Path:                   check.Path,
					Protocol:               check.Protocol,
					PortLabel:              check.PortLabel,
					Expose:                 check.Expose,
					AddressMode:            check.AddressMode,
					Interval:               check.Interval,
					Timeout:                check.Timeout,
					InitialStatus:          check.InitialStatus,
					TLSServerName:          check.TLSServerName,
					TLSSkipVerify:          check.TLSSkipVerify,
					Header:                 check.Header,
					Method:                 check.Method,
					Body:                   check.Body,
					GRPCService:            check.GRPCService,
					GRPCUseTLS:             check.GRPCUseTLS,
					SuccessBeforePassing:   check.SuccessBeforePassing,
					FailuresBeforeCritical: check.FailuresBeforeCritical,
					OnUpdate:               onUpdate,
				}

				if group {
					// only copy over task name for group level checks
					out[i].Checks[j].TaskName = check.TaskName
				}

				if check.CheckRestart != nil {
					out[i].Checks[j].CheckRestart = &structs.CheckRestart{
						Limit:          check.CheckRestart.Limit,
						Grace:          *check.CheckRestart.Grace,
						IgnoreWarnings: check.CheckRestart.IgnoreWarnings,
					}
				}
			}
		}

		if s.Connect != nil {
			out[i].Connect = ApiConsulConnectToStructs(s.Connect)
		}

		if s.Identity != nil {
			out[i].Identity = apiWorkloadIdentityToStructs(s.Identity)
		}

	}

	return out
}

func apiWorkloadIdentityToStructs(in *api.WorkloadIdentity) *structs.WorkloadIdentity {
	if in == nil {
		return nil
	}
	return &structs.WorkloadIdentity{
		Name:        in.Name,
		Audience:    in.Audience,
		Env:         in.Env,
		File:        in.File,
		ServiceName: in.ServiceName,
	}
}

func ApiConsulConnectToStructs(in *api.ConsulConnect) *structs.ConsulConnect {
	if in == nil {
		return nil
	}
	return &structs.ConsulConnect{
		Native:         in.Native,
		SidecarService: apiConnectSidecarServiceToStructs(in.SidecarService),
		SidecarTask:    apiConnectSidecarTaskToStructs(in.SidecarTask),
		Gateway:        apiConnectGatewayToStructs(in.Gateway),
	}
}

func apiConnectGatewayToStructs(in *api.ConsulGateway) *structs.ConsulGateway {
	if in == nil {
		return nil
	}

	return &structs.ConsulGateway{
		Proxy:       apiConnectGatewayProxyToStructs(in.Proxy),
		Ingress:     apiConnectIngressGatewayToStructs(in.Ingress),
		Terminating: apiConnectTerminatingGatewayToStructs(in.Terminating),
		Mesh:        apiConnectMeshGatewayToStructs(in.Mesh),
	}
}

func apiConnectGatewayProxyToStructs(in *api.ConsulGatewayProxy) *structs.ConsulGatewayProxy {
	if in == nil {
		return nil
	}

	bindAddresses := make(map[string]*structs.ConsulGatewayBindAddress)
	if in.EnvoyGatewayBindAddresses != nil {
		for k, v := range in.EnvoyGatewayBindAddresses {
			bindAddresses[k] = &structs.ConsulGatewayBindAddress{
				Address: v.Address,
				Port:    v.Port,
			}
		}
	}

	return &structs.ConsulGatewayProxy{
		ConnectTimeout:                  in.ConnectTimeout,
		EnvoyGatewayBindTaggedAddresses: in.EnvoyGatewayBindTaggedAddresses,
		EnvoyGatewayBindAddresses:       bindAddresses,
		EnvoyGatewayNoDefaultBind:       in.EnvoyGatewayNoDefaultBind,
		EnvoyDNSDiscoveryType:           in.EnvoyDNSDiscoveryType,
		Config:                          maps.Clone(in.Config),
	}
}

func apiConnectIngressGatewayToStructs(in *api.ConsulIngressConfigEntry) *structs.ConsulIngressConfigEntry {
	if in == nil {
		return nil
	}

	return &structs.ConsulIngressConfigEntry{
		TLS:       apiConnectGatewayTLSConfig(in.TLS),
		Listeners: apiConnectIngressListenersToStructs(in.Listeners),
	}
}

func apiConnectGatewayTLSConfig(in *api.ConsulGatewayTLSConfig) *structs.ConsulGatewayTLSConfig {
	if in == nil {
		return nil
	}

	return &structs.ConsulGatewayTLSConfig{
		Enabled:       in.Enabled,
		TLSMinVersion: in.TLSMinVersion,
		TLSMaxVersion: in.TLSMaxVersion,
		CipherSuites:  slices.Clone(in.CipherSuites),
	}
}

func apiConnectIngressListenersToStructs(in []*api.ConsulIngressListener) []*structs.ConsulIngressListener {
	if len(in) == 0 {
		return nil
	}

	listeners := make([]*structs.ConsulIngressListener, len(in))
	for i, listener := range in {
		listeners[i] = apiConnectIngressListenerToStructs(listener)
	}
	return listeners
}

func apiConnectIngressListenerToStructs(in *api.ConsulIngressListener) *structs.ConsulIngressListener {
	if in == nil {
		return nil
	}

	return &structs.ConsulIngressListener{
		Port:     in.Port,
		Protocol: in.Protocol,
		Services: apiConnectIngressServicesToStructs(in.Services),
	}
}

func apiConnectIngressServicesToStructs(in []*api.ConsulIngressService) []*structs.ConsulIngressService {
	if len(in) == 0 {
		return nil
	}

	services := make([]*structs.ConsulIngressService, len(in))
	for i, service := range in {
		services[i] = apiConnectIngressServiceToStructs(service)
	}
	return services
}

func apiConnectIngressServiceToStructs(in *api.ConsulIngressService) *structs.ConsulIngressService {
	if in == nil {
		return nil
	}

	return &structs.ConsulIngressService{
		Name:  in.Name,
		Hosts: slices.Clone(in.Hosts),
	}
}

func apiConnectTerminatingGatewayToStructs(in *api.ConsulTerminatingConfigEntry) *structs.ConsulTerminatingConfigEntry {
	if in == nil {
		return nil
	}

	return &structs.ConsulTerminatingConfigEntry{
		Services: apiConnectTerminatingServicesToStructs(in.Services),
	}
}

func apiConnectTerminatingServicesToStructs(in []*api.ConsulLinkedService) []*structs.ConsulLinkedService {
	if len(in) == 0 {
		return nil
	}

	services := make([]*structs.ConsulLinkedService, len(in))
	for i, service := range in {
		services[i] = apiConnectTerminatingServiceToStructs(service)
	}
	return services
}

func apiConnectTerminatingServiceToStructs(in *api.ConsulLinkedService) *structs.ConsulLinkedService {
	if in == nil {
		return nil
	}

	return &structs.ConsulLinkedService{
		Name:     in.Name,
		CAFile:   in.CAFile,
		CertFile: in.CertFile,
		KeyFile:  in.KeyFile,
		SNI:      in.SNI,
	}
}

func apiConnectMeshGatewayToStructs(in *api.ConsulMeshConfigEntry) *structs.ConsulMeshConfigEntry {
	if in == nil {
		return nil
	}
	return new(structs.ConsulMeshConfigEntry)
}

func apiConnectSidecarServiceToStructs(in *api.ConsulSidecarService) *structs.ConsulSidecarService {
	if in == nil {
		return nil
	}
	return &structs.ConsulSidecarService{
		Port:                   in.Port,
		Tags:                   slices.Clone(in.Tags),
		Proxy:                  apiConnectSidecarServiceProxyToStructs(in.Proxy),
		DisableDefaultTCPCheck: in.DisableDefaultTCPCheck,
		Meta:                   maps.Clone(in.Meta),
	}
}

func apiConnectSidecarServiceProxyToStructs(in *api.ConsulProxy) *structs.ConsulProxy {
	if in == nil {
		return nil
	}

	// TODO: to maintain backwards compatibility
	expose := in.Expose
	if in.ExposeConfig != nil {
		expose = in.ExposeConfig
	}

	return &structs.ConsulProxy{
		LocalServiceAddress: in.LocalServiceAddress,
		LocalServicePort:    in.LocalServicePort,
		Upstreams:           apiUpstreamsToStructs(in.Upstreams),
		Expose:              apiConsulExposeConfigToStructs(expose),
		Config:              maps.Clone(in.Config),
	}
}

func apiUpstreamsToStructs(in []*api.ConsulUpstream) []structs.ConsulUpstream {
	if len(in) == 0 {
		return nil
	}
	upstreams := make([]structs.ConsulUpstream, len(in))
	for i, upstream := range in {
		upstreams[i] = structs.ConsulUpstream{
			DestinationName:      upstream.DestinationName,
			DestinationNamespace: upstream.DestinationNamespace,
			LocalBindPort:        upstream.LocalBindPort,
			Datacenter:           upstream.Datacenter,
			LocalBindAddress:     upstream.LocalBindAddress,
			MeshGateway:          apiMeshGatewayToStructs(upstream.MeshGateway),
			Config:               maps.Clone(upstream.Config),
		}
	}
	return upstreams
}

func apiMeshGatewayToStructs(in *api.ConsulMeshGateway) structs.ConsulMeshGateway {
	var gw structs.ConsulMeshGateway
	if in != nil {
		gw.Mode = in.Mode
	}
	return gw
}

func apiConsulExposeConfigToStructs(in *api.ConsulExposeConfig) *structs.ConsulExposeConfig {
	if in == nil {
		return nil
	}

	// TODO: to maintain backwards compatibility
	paths := in.Paths
	if in.Path != nil {
		paths = in.Path
	}

	return &structs.ConsulExposeConfig{
		Paths: apiConsulExposePathsToStructs(paths),
	}
}

func apiConsulExposePathsToStructs(in []*api.ConsulExposePath) []structs.ConsulExposePath {
	if len(in) == 0 {
		return nil
	}
	paths := make([]structs.ConsulExposePath, len(in))
	for i, path := range in {
		paths[i] = structs.ConsulExposePath{
			Path:          path.Path,
			Protocol:      path.Protocol,
			LocalPathPort: path.LocalPathPort,
			ListenerPort:  path.ListenerPort,
		}
	}
	return paths
}

func apiConnectSidecarTaskToStructs(in *api.SidecarTask) *structs.SidecarTask {
	if in == nil {
		return nil
	}
	return &structs.SidecarTask{
		Name:          in.Name,
		Driver:        in.Driver,
		User:          in.User,
		Config:        in.Config,
		Env:           in.Env,
		Resources:     ApiResourcesToStructs(in.Resources),
		Meta:          in.Meta,
		ShutdownDelay: in.ShutdownDelay,
		KillSignal:    in.KillSignal,
		KillTimeout:   in.KillTimeout,
		LogConfig:     apiLogConfigToStructs(in.LogConfig),
	}
}

func apiConsulToStructs(in *api.Consul) *structs.Consul {
	if in == nil {
		return nil
	}
	return &structs.Consul{
		Namespace: in.Namespace,
	}
}

func apiLogConfigToStructs(in *api.LogConfig) *structs.LogConfig {
	if in == nil {
		return nil
	}

	return &structs.LogConfig{
		Disabled:      dereferenceBool(in.Disabled),
		MaxFiles:      dereferenceInt(in.MaxFiles),
		MaxFileSizeMB: dereferenceInt(in.MaxFileSizeMB),
	}
}

func dereferenceBool(in *bool) bool {
	if in == nil {
		return false
	}
	return *in
}

func dereferenceInt(in *int) int {
	if in == nil {
		return 0
	}
	return *in
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
		Weight:  *a1.Weight,
	}
}

func ApiSpreadToStructs(a1 *api.Spread) *structs.Spread {
	ret := &structs.Spread{}
	ret.Attribute = a1.Attribute
	ret.Weight = *a1.Weight
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

// validateEvalPriorityOpt ensures the supplied evaluation priority override
// value is within acceptable bounds.
func validateEvalPriorityOpt(priority int) HTTPCodedError {
	if priority < 1 || priority > 100 {
		return CodedError(400, "Eval priority must be between 1 and 100 inclusively")
	}
	return nil
}

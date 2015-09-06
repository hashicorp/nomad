package agent

import (
	"net/http"
	"strings"

	"github.com/hashicorp/nomad/nomad/structs"
)

func (s *HTTPServer) JobsRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	switch req.Method {
	case "GET":
		return s.jobListRequest(resp, req)
	case "PUT", "POST":
		return s.jobRegisterRequest(resp, req)
	default:
		return nil, CodedError(405, ErrInvalidMethod)
	}
}

func (s *HTTPServer) jobListRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	return nil, nil
}

func (s *HTTPServer) jobRegisterRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	return nil, nil
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
	default:
		return s.jobCRUD(resp, req, path)
	}
}

func (s *HTTPServer) jobForceEvaluate(resp http.ResponseWriter, req *http.Request,
	jobName string) (interface{}, error) {
	if req.Method != "PUT" && req.Method != "POST" {
		return nil, CodedError(405, ErrInvalidMethod)
	}
	return nil, nil
}

func (s *HTTPServer) jobAllocations(resp http.ResponseWriter, req *http.Request,
	jobName string) (interface{}, error) {
	if req.Method != "GET" {
		return nil, CodedError(405, ErrInvalidMethod)
	}
	return nil, nil
}

func (s *HTTPServer) jobEvaluations(resp http.ResponseWriter, req *http.Request,
	jobName string) (interface{}, error) {
	if req.Method != "GET" {
		return nil, CodedError(405, ErrInvalidMethod)
	}
	return nil, nil
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
	return nil, nil
}

func (s *HTTPServer) jobUpdate(resp http.ResponseWriter, req *http.Request,
	jobName string) (interface{}, error) {
	return nil, nil
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
	return out, nil
}

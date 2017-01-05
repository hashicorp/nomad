package agent

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/golang/snappy"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	allocNotFoundErr    = "allocation not found"
	resourceNotFoundErr = "resource not found"
)

func (s *HTTPServer) AllocsRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "GET" {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	args := structs.AllocListRequest{}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.AllocListResponse
	if err := s.agent.RPC("Alloc.List", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.Allocations == nil {
		out.Allocations = make([]*structs.AllocListStub, 0)
	}
	return out.Allocations, nil
}

func (s *HTTPServer) AllocSpecificRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	allocID := strings.TrimPrefix(req.URL.Path, "/v1/allocation/")
	if req.Method != "GET" {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	args := structs.AllocSpecificRequest{
		AllocID: allocID,
	}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.SingleAllocResponse
	if err := s.agent.RPC("Alloc.GetAlloc", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.Alloc == nil {
		return nil, CodedError(404, "alloc not found")
	}

	// Decode the payload if there is any
	alloc := out.Alloc
	if alloc.Job != nil && len(alloc.Job.Payload) != 0 {
		decoded, err := snappy.Decode(nil, alloc.Job.Payload)
		if err != nil {
			return nil, err
		}
		alloc = alloc.Copy()
		alloc.Job.Payload = decoded
	}

	return alloc, nil
}

func (s *HTTPServer) ClientAllocRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if s.agent.client == nil {
		return nil, clientNotRunning
	}

	reqSuffix := strings.TrimPrefix(req.URL.Path, "/v1/client/allocation/")

	// tokenize the suffix of the path to get the alloc id and find the action
	// invoked on the alloc id
	tokens := strings.Split(reqSuffix, "/")
	if len(tokens) != 2 {
		return nil, CodedError(404, resourceNotFoundErr)
	}
	allocID := tokens[0]
	switch tokens[1] {
	case "stats":
		return s.allocStats(allocID, resp, req)
	case "snapshot":
		return s.allocSnapshot(allocID, resp, req)
	case "gc":
		return s.allocGC(allocID, resp, req)
	}

	return nil, CodedError(404, resourceNotFoundErr)
}

func (s *HTTPServer) ClientGCRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if s.agent.client == nil {
		return nil, clientNotRunning
	}
	return nil, s.agent.Client().CollectAllAllocs()
}

func (s *HTTPServer) allocGC(allocID string, resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	return nil, s.agent.Client().CollectAllocation(allocID)
}

func (s *HTTPServer) allocSnapshot(allocID string, resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	allocFS, err := s.agent.Client().GetAllocFS(allocID)
	if err != nil {
		return nil, fmt.Errorf(allocNotFoundErr)
	}
	if err := allocFS.Snapshot(resp); err != nil {
		return nil, fmt.Errorf("error making snapshot: %v", err)
	}
	return nil, nil
}

func (s *HTTPServer) allocStats(allocID string, resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	clientStats := s.agent.client.StatsReporter()
	aStats, err := clientStats.GetAllocStats(allocID)
	if err != nil {
		return nil, err
	}

	task := req.URL.Query().Get("task")
	return aStats.LatestAllocStats(task)
}

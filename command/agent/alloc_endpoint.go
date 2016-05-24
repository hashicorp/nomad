package agent

import (
	"net/http"
	"strings"

	"github.com/hashicorp/nomad/nomad/structs"
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
	return out.Alloc, nil
}

func (s *HTTPServer) ClientAllocRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if s.agent.client == nil {
		return nil, clientNotRunning
	}

	reqSuffix := strings.TrimPrefix(req.URL.Path, "/v1/client/allocation/")

	// tokenize the suffix of the path to get the alloc id and find the action
	// invoked on the alloc id
	tokens := strings.Split(reqSuffix, "/")
	if len(tokens) == 1 || tokens[1] != "stats" {
		return nil, CodedError(404, "url not found")
	}
	allocID := tokens[0]

	clientStats := s.agent.client.StatsReporter()
	allocStats, ok := clientStats.AllocStats()[allocID]
	if !ok {
		return nil, CodedError(404, "alloc not running in node")
	}

	if task := req.URL.Query().Get("task"); task != "" {
		taskStats, err := allocStats.TaskStats(task)
		if err != nil {
			return nil, CodedError(404, "task not present in allocation")
		}
		return taskStats.ResourceUsage(), nil
	}

	// Return the resource usage of all the tasks in an allocation if task name
	// is not specified
	res := make(map[string]interface{})
	for task, taskStats := range allocStats.AllocStats() {
		res[task] = taskStats.ResourceUsage()
	}
	return res, nil
}

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
	return out.Allocations, nil
}

func (s *HTTPServer) AllocSpecificRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	allocID := strings.TrimPrefix(req.URL.Path, "/v1/allocation/")
	switch req.Method {
	case "GET":
		return s.allocQuery(resp, req, allocID)
	case "DELETE":
		return s.allocDelete(resp, req, allocID)
	default:
		return nil, CodedError(405, ErrInvalidMethod)
	}
}

func (s *HTTPServer) allocQuery(resp http.ResponseWriter, req *http.Request,
	allocID string) (interface{}, error) {
	// TODO
	return nil, nil
}

func (s *HTTPServer) allocDelete(resp http.ResponseWriter, req *http.Request,
	allocID string) (interface{}, error) {
	// TODO
	return nil, nil
}

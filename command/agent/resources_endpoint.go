package agent

import (
	"github.com/hashicorp/nomad/nomad/structs"
	"net/http"
)

// ResourceListRequest accepts a prefix and context and returns a list of matching
// IDs for that context.
func (s *HTTPServer) ResourceListRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method == "POST" || req.Method == "PUT" {
		return s.resourcesRequest(resp, req)
	}
	return nil, CodedError(405, ErrInvalidMethod)
}

func (s *HTTPServer) resourcesRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	args := structs.ResourceListRequest{}

	if err := decodeBody(req, &args); err != nil {
		return nil, CodedError(400, err.Error())
	}

	var out structs.ResourceListResponse
	if err := s.agent.RPC("Resources.List", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	return out, nil
}

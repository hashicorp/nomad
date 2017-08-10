package agent

import (
	"github.com/hashicorp/nomad/nomad/structs"
	"net/http"
)

// ClusterSearchRequest accepts a prefix and context and returns a list of matching
// IDs for that context.
func (s *HTTPServer) ClusterSearchRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method == "POST" || req.Method == "PUT" {
		return s.newClusterSearchRequest(resp, req)
	}
	return nil, CodedError(405, ErrInvalidMethod)
}

func (s *HTTPServer) newClusterSearchRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	args := structs.ClusterSearchRequest{}

	if err := decodeBody(req, &args); err != nil {
		return nil, CodedError(400, err.Error())
	}

	var out structs.ClusterSearchResponse
	if err := s.agent.RPC("ClusterSearch.List", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	return out, nil
}

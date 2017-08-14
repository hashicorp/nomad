package agent

import (
	"net/http"

	"github.com/hashicorp/nomad/nomad/structs"
)

// SearchRequest accepts a prefix and context and returns a list of matching
// IDs for that context.
func (s *HTTPServer) SearchRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method == "POST" || req.Method == "PUT" {
		return s.newSearchRequest(resp, req)
	}
	return nil, CodedError(405, ErrInvalidMethod)
}

func (s *HTTPServer) newSearchRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	args := structs.SearchRequest{}

	if err := decodeBody(req, &args); err != nil {
		return nil, CodedError(400, err.Error())
	}

	var out structs.SearchResponse
	if err := s.agent.RPC("Search.PrefixSearch", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	return out, nil
}

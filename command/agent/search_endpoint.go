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
	return nil, CodedError(http.StatusMethodNotAllowed, ErrInvalidMethod)
}

func (s *HTTPServer) newSearchRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	args := structs.SearchRequest{}

	if err := decodeBody(req, &args); err != nil {
		return nil, CodedError(http.StatusBadRequest, err.Error())
	}

	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.SearchResponse
	if err := s.agent.RPC("Search.PrefixSearch", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	return out, nil
}

func (s *HTTPServer) FuzzySearchRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method == "POST" || req.Method == "PUT" {
		return s.newFuzzySearchRequest(resp, req)
	}
	return nil, CodedError(http.StatusMethodNotAllowed, ErrInvalidMethod)
}

func (s *HTTPServer) newFuzzySearchRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var args structs.FuzzySearchRequest

	if err := decodeBody(req, &args); err != nil {
		return nil, CodedError(http.StatusBadRequest, err.Error())
	}

	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.FuzzySearchResponse
	if err := s.agent.RPC("Search.FuzzySearch", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	return out, nil
}

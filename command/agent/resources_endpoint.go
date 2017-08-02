package agent

import (
	"github.com/hashicorp/nomad/nomad/structs"
	"net/http"
)

func (s *HTTPServer) ResourcesRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	switch req.Method {
	case "POST":
		return s.resourcesRequest(resp, req)
	default:
		return nil, CodedError(405, ErrInvalidMethod)
	}
}

func (s *HTTPServer) resourcesRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// TODO test a failure case for this?
	args := structs.ResourcesRequest{}

	// TODO this should be tested
	if err := decodeBody(req, &args); err != nil {
		return nil, CodedError(400, err.Error())
	}

	var out structs.ResourcesResponse
	if err := s.agent.RPC("Resources.List", &args, &out); err != nil {
		return nil, err
	}

	return out, nil
}

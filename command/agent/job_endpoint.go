package agent

import (
	"fmt"
	"net/http"

	"github.com/hashicorp/consul/consul/structs"
)

func (s *HTTPServer) JobsList(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var args structs.RegisterRequest
	if err := decodeBody(req, &args, nil); err != nil {
		resp.WriteHeader(400)
		resp.Write([]byte(fmt.Sprintf("Request decode failed: %v", err)))
		return nil, nil
	}

	// Setup the default DC if not provided
	if args.Datacenter == "" {
		args.Datacenter = s.agent.config.Datacenter
	}

	// Forward to the servers
	var out struct{}
	if err := s.agent.RPC("Catalog.Register", &args, &out); err != nil {
		return nil, err
	}
	return true, nil
}

func (s *HTTPServer) JobSpecificRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var args structs.RegisterRequest
	if err := decodeBody(req, &args, nil); err != nil {
		resp.WriteHeader(400)
		resp.Write([]byte(fmt.Sprintf("Request decode failed: %v", err)))
		return nil, nil
	}

	// Setup the default DC if not provided
	if args.Datacenter == "" {
		args.Datacenter = s.agent.config.Datacenter
	}

	// Forward to the servers
	var out struct{}
	if err := s.agent.RPC("Catalog.Register", &args, &out); err != nil {
		return nil, err
	}
	return true, nil
}

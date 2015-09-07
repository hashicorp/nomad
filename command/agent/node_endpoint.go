package agent

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/hashicorp/nomad/nomad/structs"
)

func (s *HTTPServer) NodesRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "GET" {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	args := structs.NodeListRequest{}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.NodeListResponse
	if err := s.agent.RPC("Client.List", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	return out.Nodes, nil
}

func (s *HTTPServer) NodeSpecificRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	path := strings.TrimPrefix(req.URL.Path, "/v1/node/")
	switch {
	case strings.HasSuffix(path, "/evaluate"):
		nodeName := strings.TrimSuffix(path, "/evaluate")
		return s.nodeForceEvaluate(resp, req, nodeName)
	case strings.HasSuffix(path, "/allocations"):
		nodeName := strings.TrimSuffix(path, "/allocations")
		return s.nodeAllocations(resp, req, nodeName)
	case strings.HasSuffix(path, "/drain"):
		nodeName := strings.TrimSuffix(path, "/drain")
		return s.nodeToggleDrain(resp, req, nodeName)
	default:
		return s.nodeQuery(resp, req, path)
	}
}

func (s *HTTPServer) nodeForceEvaluate(resp http.ResponseWriter, req *http.Request,
	nodeID string) (interface{}, error) {
	if req.Method != "PUT" && req.Method != "POST" {
		return nil, CodedError(405, ErrInvalidMethod)
	}
	args := structs.NodeEvaluateRequest{
		NodeID: nodeID,
	}
	s.parseRegion(req, &args.Region)

	var out structs.NodeUpdateResponse
	if err := s.agent.RPC("Client.Evaluate", &args, &out); err != nil {
		return nil, err
	}
	setIndex(resp, out.Index)
	return out, nil
}

func (s *HTTPServer) nodeAllocations(resp http.ResponseWriter, req *http.Request,
	nodeID string) (interface{}, error) {
	if req.Method != "GET" {
		return nil, CodedError(405, ErrInvalidMethod)
	}
	args := structs.NodeSpecificRequest{
		NodeID: nodeID,
	}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.NodeAllocsResponse
	if err := s.agent.RPC("Client.GetAllocs", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	return out.Allocs, nil
}

func (s *HTTPServer) nodeToggleDrain(resp http.ResponseWriter, req *http.Request,
	nodeID string) (interface{}, error) {
	if req.Method != "PUT" && req.Method != "POST" {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	// Get the enable value
	enableRaw := req.URL.Query().Get("enable")
	if enableRaw == "" {
		return nil, CodedError(400, "missing enable value")
	}
	enable, err := strconv.ParseBool(enableRaw)
	if err != nil {
		return nil, CodedError(400, "invalid enable value")
	}

	args := structs.NodeUpdateDrainRequest{
		NodeID: nodeID,
		Drain:  enable,
	}
	s.parseRegion(req, &args.Region)

	var out structs.NodeDrainUpdateResponse
	if err := s.agent.RPC("Client.UpdateDrain", &args, &out); err != nil {
		return nil, err
	}
	setIndex(resp, out.Index)
	return out, nil
}

func (s *HTTPServer) nodeQuery(resp http.ResponseWriter, req *http.Request,
	nodeID string) (interface{}, error) {
	if req.Method != "GET" {
		return nil, CodedError(405, ErrInvalidMethod)
	}
	args := structs.NodeSpecificRequest{
		NodeID: nodeID,
	}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.SingleNodeResponse
	if err := s.agent.RPC("Client.GetNode", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.Node == nil {
		return nil, CodedError(404, "node not found")
	}
	return out.Node, nil
}

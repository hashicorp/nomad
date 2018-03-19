package agent

import (
	"net/http"
	"strings"

	"github.com/hashicorp/nomad/api"
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
	if err := s.agent.RPC("Node.List", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.Nodes == nil {
		out.Nodes = make([]*structs.NodeListStub, 0)
	}
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
	case strings.HasSuffix(path, "/purge"):
		nodeName := strings.TrimSuffix(path, "/purge")
		return s.nodePurge(resp, req, nodeName)
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
	s.parseWriteRequest(req, &args.WriteRequest)

	var out structs.NodeUpdateResponse
	if err := s.agent.RPC("Node.Evaluate", &args, &out); err != nil {
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
	if err := s.agent.RPC("Node.GetAllocs", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.Allocs == nil {
		out.Allocs = make([]*structs.Allocation, 0)
	}
	for _, alloc := range out.Allocs {
		alloc.SetEventDisplayMessages()
	}
	return out.Allocs, nil
}

func (s *HTTPServer) nodeToggleDrain(resp http.ResponseWriter, req *http.Request,
	nodeID string) (interface{}, error) {
	if req.Method != "PUT" && req.Method != "POST" {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	var drainRequest api.NodeUpdateDrainRequest
	if err := decodeBody(req, &drainRequest); err != nil {
		return nil, CodedError(400, err.Error())
	}

	args := structs.NodeUpdateDrainRequest{
		NodeID: nodeID,
	}
	if drainRequest.DrainSpec != nil {
		args.DrainStrategy = &structs.DrainStrategy{
			DrainSpec: structs.DrainSpec{
				Deadline:         drainRequest.DrainSpec.Deadline,
				IgnoreSystemJobs: drainRequest.DrainSpec.IgnoreSystemJobs,
			},
		}
	}
	s.parseWriteRequest(req, &args.WriteRequest)

	var out structs.NodeDrainUpdateResponse
	if err := s.agent.RPC("Node.UpdateDrain", &args, &out); err != nil {
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
	if err := s.agent.RPC("Node.GetNode", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.Node == nil {
		return nil, CodedError(404, "node not found")
	}
	return out.Node, nil
}

func (s *HTTPServer) nodePurge(resp http.ResponseWriter, req *http.Request, nodeID string) (interface{}, error) {
	if req.Method != "PUT" && req.Method != "POST" {
		return nil, CodedError(405, ErrInvalidMethod)
	}
	args := structs.NodeDeregisterRequest{
		NodeID: nodeID,
	}
	s.parseWriteRequest(req, &args.WriteRequest)
	var out structs.NodeUpdateResponse
	if err := s.agent.RPC("Node.Deregister", &args, &out); err != nil {
		return nil, err
	}
	setIndex(resp, out.Index)
	return out, nil
}

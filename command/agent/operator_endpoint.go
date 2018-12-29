package agent

import (
	"net/http"
	"strings"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/raft"
)

func (s *HTTPServer) OperatorRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	path := strings.TrimPrefix(req.URL.Path, "/v1/operator/raft/")
	switch {
	case strings.HasPrefix(path, "configuration"):
		return s.OperatorRaftConfiguration(resp, req)
	case strings.HasPrefix(path, "peer"):
		return s.OperatorRaftPeer(resp, req)
	default:
		return nil, CodedError(404, ErrInvalidMethod)
	}
}

// OperatorRaftConfiguration is used to inspect the current Raft configuration.
// This supports the stale query mode in case the cluster doesn't have a leader.
func (s *HTTPServer) OperatorRaftConfiguration(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "GET" {
		resp.WriteHeader(http.StatusMethodNotAllowed)
		return nil, nil
	}

	var args structs.GenericRequest
	if done := s.parse(resp, req, &args.Region, &args.QueryOptions); done {
		return nil, nil
	}

	var reply structs.RaftConfigurationResponse
	if err := s.agent.RPC("Operator.RaftGetConfiguration", &args, &reply); err != nil {
		return nil, err
	}

	return reply, nil
}

// OperatorRaftPeer supports actions on Raft peers. Currently we only support
// removing peers by address.
func (s *HTTPServer) OperatorRaftPeer(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "DELETE" {
		resp.WriteHeader(http.StatusMethodNotAllowed)
		return nil, nil
	}

	var args structs.RaftPeerByAddressRequest
	s.parseWriteRequest(req, &args.WriteRequest)

	params := req.URL.Query()
	if _, ok := params["address"]; ok {
		args.Address = raft.ServerAddress(params.Get("address"))
	} else {
		resp.WriteHeader(http.StatusBadRequest)
		resp.Write([]byte("Must specify ?address with IP:port of peer to remove"))
		return nil, nil
	}

	var reply struct{}
	if err := s.agent.RPC("Operator.RaftRemovePeerByAddress", &args, &reply); err != nil {
		return nil, err
	}
	return nil, nil
}

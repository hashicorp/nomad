package agent

import (
	"net/http"

	"github.com/hashicorp/serf/serf"
)

func (s *HTTPServer) AgentSelfRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "GET" {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	// Get the member as a server
	var member *serf.Member
	srv := s.agent.Server()
	if srv != nil {
		mem := srv.LocalMember()
		member = &mem
	}

	self := agentSelf{
		Config: s.agent.config,
		Member: member,
		Stats:  s.agent.Stats(),
	}
	return self, nil
}

func (s *HTTPServer) AgentJoinRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "PUT" && req.Method != "POST" {
		return nil, CodedError(405, ErrInvalidMethod)
	}
	srv := s.agent.Server()
	if srv == nil {
		return nil, CodedError(501, ErrInvalidMethod)
	}

	// Get the join addresses
	query := req.URL.Query()
	addrs := query["address"]
	if len(addrs) == 0 {
		return nil, CodedError(400, "missing address to join")
	}

	// Attempt the join
	num, err := srv.Join(addrs)
	var errStr string
	if err != nil {
		errStr = err.Error()
	}
	return joinResult{num, errStr}, nil
}

func (s *HTTPServer) AgentMembersRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "GET" {
		return nil, CodedError(405, ErrInvalidMethod)
	}
	srv := s.agent.Server()
	if srv == nil {
		return nil, CodedError(501, ErrInvalidMethod)
	}
	return srv.Members(), nil
}

func (s *HTTPServer) AgentForceLeaveRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "PUT" && req.Method != "POST" {
		return nil, CodedError(405, ErrInvalidMethod)
	}
	srv := s.agent.Server()
	if srv == nil {
		return nil, CodedError(501, ErrInvalidMethod)
	}

	// Get the node to eject
	node := req.URL.Query().Get("node")
	if node == "" {
		return nil, CodedError(400, "missing node to force leave")
	}

	// Attempt remove
	err := srv.RemoveFailedNode(node)
	return nil, err
}

type agentSelf struct {
	Config *Config                      `json:"config"`
	Member *serf.Member                 `json:"member,omitempty"`
	Stats  map[string]map[string]string `json:"stats"`
}

type joinResult struct {
	NumJoined int    `json:"num_joined"`
	Error     string `json:"error"`
}

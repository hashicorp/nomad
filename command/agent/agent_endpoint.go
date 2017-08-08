package agent

import (
	"net"
	"net/http"
	"strings"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/serf/serf"
	"github.com/mitchellh/copystructure"
)

type Member struct {
	Name        string
	Addr        net.IP
	Port        uint16
	Tags        map[string]string
	Status      string
	ProtocolMin uint8
	ProtocolMax uint8
	ProtocolCur uint8
	DelegateMin uint8
	DelegateMax uint8
	DelegateCur uint8
}

func nomadMember(m serf.Member) Member {
	return Member{
		Name:        m.Name,
		Addr:        m.Addr,
		Port:        m.Port,
		Tags:        m.Tags,
		Status:      m.Status.String(),
		ProtocolMin: m.ProtocolMin,
		ProtocolMax: m.ProtocolMax,
		ProtocolCur: m.ProtocolCur,
		DelegateMin: m.DelegateMin,
		DelegateMax: m.DelegateMax,
		DelegateCur: m.DelegateCur,
	}
}

func (s *HTTPServer) AgentSelfRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "GET" {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	// Get the member as a server
	var member serf.Member
	srv := s.agent.Server()
	if srv != nil {
		member = srv.LocalMember()
	}

	self := agentSelf{
		Member: nomadMember(member),
		Stats:  s.agent.Stats(),
	}
	if ac, err := copystructure.Copy(s.agent.config); err != nil {
		return nil, CodedError(500, err.Error())
	} else {
		self.Config = ac.(*Config)
	}

	if self.Config != nil && self.Config.Vault != nil && self.Config.Vault.Token != "" {
		self.Config.Vault.Token = "<redacted>"
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
	args := &structs.GenericRequest{}
	var out structs.ServerMembersResponse
	if err := s.agent.RPC("Status.Members", args, &out); err != nil {
		return nil, err
	}

	return out, nil
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

// AgentServersRequest is used to query the list of servers used by the Nomad
// Client for RPCs.  This endpoint can also be used to update the list of
// servers for a given agent.
func (s *HTTPServer) AgentServersRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	switch req.Method {
	case "PUT", "POST":
		return s.updateServers(resp, req)
	case "GET":
		return s.listServers(resp, req)
	default:
		return nil, CodedError(405, ErrInvalidMethod)
	}
}

func (s *HTTPServer) listServers(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	client := s.agent.Client()
	if client == nil {
		return nil, CodedError(501, ErrInvalidMethod)
	}

	peers := s.agent.client.GetServers()
	return peers, nil
}

func (s *HTTPServer) updateServers(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	client := s.agent.Client()
	if client == nil {
		return nil, CodedError(501, ErrInvalidMethod)
	}

	// Get the servers from the request
	servers := req.URL.Query()["address"]
	if len(servers) == 0 {
		return nil, CodedError(400, "missing server address")
	}

	// Set the servers list into the client
	s.agent.logger.Printf("[TRACE] Adding servers %+q to the client's primary server list", servers)
	if err := client.SetServers(servers); err != nil {
		s.agent.logger.Printf("[ERR] Attempt to add servers %q to client failed: %v", servers, err)
		//TODO is this the right error to return?
		return nil, CodedError(400, err.Error())
	}
	return nil, nil
}

// KeyringOperationRequest allows an operator to install/delete/use keys
func (s *HTTPServer) KeyringOperationRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	srv := s.agent.Server()
	if srv == nil {
		return nil, CodedError(501, ErrInvalidMethod)
	}

	kmgr := srv.KeyManager()
	var sresp *serf.KeyResponse
	var err error

	// Get the key from the req body
	var args structs.KeyringRequest

	//Get the op
	op := strings.TrimPrefix(req.URL.Path, "/v1/agent/keyring/")

	switch op {
	case "list":
		sresp, err = kmgr.ListKeys()
	case "install":
		if err := decodeBody(req, &args); err != nil {
			return nil, CodedError(500, err.Error())
		}
		sresp, err = kmgr.InstallKey(args.Key)
	case "use":
		if err := decodeBody(req, &args); err != nil {
			return nil, CodedError(500, err.Error())
		}
		sresp, err = kmgr.UseKey(args.Key)
	case "remove":
		if err := decodeBody(req, &args); err != nil {
			return nil, CodedError(500, err.Error())
		}
		sresp, err = kmgr.RemoveKey(args.Key)
	default:
		return nil, CodedError(404, "resource not found")
	}

	if err != nil {
		return nil, err
	}
	kresp := structs.KeyringResponse{
		Messages: sresp.Messages,
		Keys:     sresp.Keys,
		NumNodes: sresp.NumNodes,
	}
	return kresp, nil
}

type agentSelf struct {
	Config *Config                      `json:"config"`
	Member Member                       `json:"member,omitempty"`
	Stats  map[string]map[string]string `json:"stats"`
}

type joinResult struct {
	NumJoined int    `json:"num_joined"`
	Error     string `json:"error"`
}

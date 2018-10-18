package agent

import (
	"encoding/json"
	"net"
	"net/http"
	"sort"
	"strings"

	"github.com/hashicorp/nomad/acl"
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

	var secret string
	s.parseToken(req, &secret)

	var aclObj *acl.ACL
	var err error

	// Get the member as a server
	var member serf.Member
	if srv := s.agent.Server(); srv != nil {
		member = srv.LocalMember()
		aclObj, err = srv.ResolveToken(secret)
	} else {
		// Not a Server; use the Client for token resolution
		aclObj, err = s.agent.Client().ResolveToken(secret)
	}

	if err != nil {
		return nil, err
	}

	// Check agent read permissions
	if aclObj != nil && !aclObj.AllowAgentRead() {
		return nil, structs.ErrPermissionDenied
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
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

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

	var secret string
	s.parseToken(req, &secret)

	// Check agent write permissions
	if aclObj, err := s.agent.Server().ResolveToken(secret); err != nil {
		return nil, err
	} else if aclObj != nil && !aclObj.AllowAgentWrite() {
		return nil, structs.ErrPermissionDenied
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

	var secret string
	s.parseToken(req, &secret)

	// Check agent read permissions
	if aclObj, err := s.agent.Client().ResolveToken(secret); err != nil {
		return nil, err
	} else if aclObj != nil && !aclObj.AllowAgentRead() {
		return nil, structs.ErrPermissionDenied
	}

	peers := s.agent.client.GetServers()
	sort.Strings(peers)
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

	var secret string
	s.parseToken(req, &secret)

	// Check agent write permissions
	if aclObj, err := s.agent.Client().ResolveToken(secret); err != nil {
		return nil, err
	} else if aclObj != nil && !aclObj.AllowAgentWrite() {
		return nil, structs.ErrPermissionDenied
	}

	// Set the servers list into the client
	s.agent.logger.Trace("adding servers to the client's primary server list", "servers", servers, "path", "/v1/agent/servers", "method", "PUT")
	if _, err := client.SetServers(servers); err != nil {
		s.agent.logger.Error("failed adding servers to client's server list", "servers", servers, "error", err, "path", "/v1/agent/servers", "method", "PUT")
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

	var secret string
	s.parseToken(req, &secret)

	// Check agent write permissions
	if aclObj, err := srv.ResolveToken(secret); err != nil {
		return nil, err
	} else if aclObj != nil && !aclObj.AllowAgentWrite() {
		return nil, structs.ErrPermissionDenied
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

func (s *HTTPServer) HealthRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "GET" {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	var args structs.GenericRequest
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	health := healthResponse{}
	getClient := true
	getServer := true

	// See if we're checking a specific agent type and default to failing
	if healthType, ok := req.URL.Query()["type"]; ok {
		getClient = false
		getServer = false
		for _, ht := range healthType {
			switch ht {
			case "client":
				getClient = true
				health.Client = &healthResponseAgent{
					Ok:      false,
					Message: "client not enabled",
				}
			case "server":
				getServer = true
				health.Server = &healthResponseAgent{
					Ok:      false,
					Message: "server not enabled",
				}
			}
		}
	}

	// If we should check the client and it exists assume it's healthy
	if client := s.agent.Client(); getClient && client != nil {
		if len(client.GetServers()) == 0 {
			health.Client = &healthResponseAgent{
				Ok:      false,
				Message: "no known servers",
			}
		} else {
			health.Client = &healthResponseAgent{
				Ok:      true,
				Message: "ok",
			}
		}
	}

	// If we should check the server and it exists, see if there's a leader
	if server := s.agent.Server(); getServer && server != nil {
		health.Server = &healthResponseAgent{
			Ok:      true,
			Message: "ok",
		}

		leader := ""
		if err := s.agent.RPC("Status.Leader", &args, &leader); err != nil {
			health.Server.Ok = false
			health.Server.Message = err.Error()
		} else if leader == "" {
			health.Server.Ok = false
			health.Server.Message = "no leader"
		}
	}

	if health.ok() {
		return &health, nil
	}

	jsonResp, err := json.Marshal(&health)
	if err != nil {
		return nil, err
	}
	return nil, CodedError(500, string(jsonResp))
}

type healthResponse struct {
	Client *healthResponseAgent `json:"client,omitempty"`
	Server *healthResponseAgent `json:"server,omitempty"`
}

// ok returns true as long as neither Client nor Server have Ok=false.
func (h healthResponse) ok() bool {
	if h.Client != nil && !h.Client.Ok {
		return false
	}
	if h.Server != nil && !h.Server.Ok {
		return false
	}
	return true
}

type healthResponseAgent struct {
	Ok      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}

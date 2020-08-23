package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/docker/docker/pkg/ioutils"
	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-msgpack/codec"
	"github.com/hashicorp/nomad/acl"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/command/agent/host"
	"github.com/hashicorp/nomad/command/agent/pprof"
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

	if self.Config != nil && self.Config.ACL != nil && self.Config.ACL.ReplicationToken != "" {
		self.Config.ACL.ReplicationToken = "<redacted>"
	}

	if self.Config != nil && self.Config.Consul != nil && self.Config.Consul.Token != "" {
		self.Config.Consul.Token = "<redacted>"
	}

	if self.Config != nil && self.Config.Telemetry != nil && self.Config.Telemetry.CirconusAPIToken != "" {
		self.Config.Telemetry.CirconusAPIToken = "<redacted>"
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

func (s *HTTPServer) AgentMonitor(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Get the provided loglevel.
	logLevel := req.URL.Query().Get("log_level")
	if logLevel == "" {
		logLevel = "INFO"
	}

	if log.LevelFromString(logLevel) == log.NoLevel {
		return nil, CodedError(400, fmt.Sprintf("Unknown log level: %s", logLevel))
	}

	logJSON := false
	logJSONStr := req.URL.Query().Get("log_json")
	if logJSONStr != "" {
		parsed, err := strconv.ParseBool(logJSONStr)
		if err != nil {
			return nil, CodedError(400, fmt.Sprintf("Unknown option for log json: %v", err))
		}
		logJSON = parsed
	}

	plainText := false
	plainTextStr := req.URL.Query().Get("plain")
	if plainTextStr != "" {
		parsed, err := strconv.ParseBool(plainTextStr)
		if err != nil {
			return nil, CodedError(400, fmt.Sprintf("Unknown option for plain: %v", err))
		}
		plainText = parsed
	}

	nodeID := req.URL.Query().Get("node_id")
	// Build the request and parse the ACL token
	args := cstructs.MonitorRequest{
		NodeID:    nodeID,
		ServerID:  req.URL.Query().Get("server_id"),
		LogLevel:  logLevel,
		LogJSON:   logJSON,
		PlainText: plainText,
	}

	// if node and server were requested return error
	if args.NodeID != "" && args.ServerID != "" {
		return nil, CodedError(400, "Cannot target node and server simultaneously")
	}

	// Force the Content-Type to avoid Go's http.ResponseWriter from
	// detecting an incorrect or unsafe one.
	if plainText {
		resp.Header().Set("Content-Type", "text/plain")
	} else {
		resp.Header().Set("Content-Type", "application/json")
	}

	s.parse(resp, req, &args.QueryOptions.Region, &args.QueryOptions)

	// Make the RPC
	var handler structs.StreamingRpcHandler
	var handlerErr error
	if nodeID != "" {
		// Determine the handler to use
		useLocalClient, useClientRPC, useServerRPC := s.rpcHandlerForNode(nodeID)
		if useLocalClient {
			handler, handlerErr = s.agent.Client().StreamingRpcHandler("Agent.Monitor")
		} else if useClientRPC {
			handler, handlerErr = s.agent.Client().RemoteStreamingRpcHandler("Agent.Monitor")
		} else if useServerRPC {
			handler, handlerErr = s.agent.Server().StreamingRpcHandler("Agent.Monitor")
		} else {
			handlerErr = CodedError(400, "No local Node and node_id not provided")
		}
		// No node id monitor current server/client
	} else if srv := s.agent.Server(); srv != nil {
		handler, handlerErr = srv.StreamingRpcHandler("Agent.Monitor")
	} else {
		handler, handlerErr = s.agent.Client().StreamingRpcHandler("Agent.Monitor")
	}

	if handlerErr != nil {
		return nil, CodedError(500, handlerErr.Error())
	}
	httpPipe, handlerPipe := net.Pipe()
	decoder := codec.NewDecoder(httpPipe, structs.MsgpackHandle)
	encoder := codec.NewEncoder(httpPipe, structs.MsgpackHandle)

	ctx, cancel := context.WithCancel(req.Context())
	go func() {
		<-ctx.Done()
		httpPipe.Close()
	}()

	// Create an output that gets flushed on every write
	output := ioutils.NewWriteFlusher(resp)

	// create an error channel to handle errors
	errCh := make(chan HTTPCodedError, 2)

	// stream response
	go func() {
		defer cancel()

		// Send the request
		if err := encoder.Encode(args); err != nil {
			errCh <- CodedError(500, err.Error())
			return
		}

		for {
			select {
			case <-ctx.Done():
				errCh <- nil
				return
			default:
			}

			var res cstructs.StreamErrWrapper
			if err := decoder.Decode(&res); err != nil {
				errCh <- CodedError(500, err.Error())
				return
			}
			decoder.Reset(httpPipe)

			if err := res.Error; err != nil {
				if err.Code != nil {
					errCh <- CodedError(int(*err.Code), err.Error())
					return
				}
			}

			if _, err := io.Copy(output, bytes.NewReader(res.Payload)); err != nil {
				errCh <- CodedError(500, err.Error())
				return
			}
		}
	}()

	handler(handlerPipe)
	cancel()
	codedErr := <-errCh

	if codedErr != nil &&
		(codedErr == io.EOF ||
			strings.Contains(codedErr.Error(), "closed") ||
			strings.Contains(codedErr.Error(), "EOF")) {
		codedErr = nil
	}
	return nil, codedErr
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

func (s *HTTPServer) AgentPprofRequest(resp http.ResponseWriter, req *http.Request) ([]byte, error) {
	path := strings.TrimPrefix(req.URL.Path, "/v1/agent/pprof/")
	switch path {
	case "":
		// no root index route
		return nil, CodedError(404, ErrInvalidMethod)
	case "cmdline":
		return s.agentPprof(pprof.CmdReq, resp, req)
	case "profile":
		return s.agentPprof(pprof.CPUReq, resp, req)
	case "trace":
		return s.agentPprof(pprof.TraceReq, resp, req)
	default:
		// Add profile to request
		values := req.URL.Query()
		values.Add("profile", path)
		req.URL.RawQuery = values.Encode()

		// generic pprof profile request
		return s.agentPprof(pprof.LookupReq, resp, req)
	}
}

func (s *HTTPServer) agentPprof(reqType pprof.ReqType, resp http.ResponseWriter, req *http.Request) ([]byte, error) {

	// Parse query param int values
	// Errors are dropped here and default to their zero values.
	// This is to mimick the functionality that net/pprof implements.
	seconds, _ := strconv.Atoi(req.URL.Query().Get("seconds"))
	debug, _ := strconv.Atoi(req.URL.Query().Get("debug"))
	gc, _ := strconv.Atoi(req.URL.Query().Get("gc"))

	// default to 1 second
	if seconds == 0 {
		seconds = 1
	}

	// Create the request
	args := &structs.AgentPprofRequest{
		NodeID:   req.URL.Query().Get("node_id"),
		Profile:  req.URL.Query().Get("profile"),
		ServerID: req.URL.Query().Get("server_id"),
		Debug:    debug,
		GC:       gc,
		ReqType:  reqType,
		Seconds:  seconds,
	}

	// if node and server were requested return error
	if args.NodeID != "" && args.ServerID != "" {
		return nil, CodedError(400, "Cannot target node and server simultaneously")
	}

	s.parse(resp, req, &args.QueryOptions.Region, &args.QueryOptions)

	var reply structs.AgentPprofResponse
	var rpcErr error
	if args.NodeID != "" {
		// Make the RPC
		localClient, remoteClient, localServer := s.rpcHandlerForNode(args.NodeID)
		if localClient {
			rpcErr = s.agent.Client().ClientRPC("Agent.Profile", &args, &reply)
		} else if remoteClient {
			rpcErr = s.agent.Client().RPC("Agent.Profile", &args, &reply)
		} else if localServer {
			rpcErr = s.agent.Server().RPC("Agent.Profile", &args, &reply)
		}
		// No node id, profile current server/client
	} else if srv := s.agent.Server(); srv != nil {
		rpcErr = srv.RPC("Agent.Profile", &args, &reply)
	} else {
		rpcErr = s.agent.Client().RPC("Agent.Profile", &args, &reply)
	}

	if rpcErr != nil {
		return nil, rpcErr
	}

	// Set headers from profile request
	for k, v := range reply.HTTPHeaders {
		resp.Header().Set(k, v)
	}

	return reply.Payload, nil
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

// AgentHostRequest runs on servers and clients, and captures information about the host system to add
// to the nomad operator debug archive.
func (s *HTTPServer) AgentHostRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != http.MethodGet {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	var secret string
	s.parseToken(req, &secret)

	// Check agent read permissions
	var aclObj *acl.ACL
	var enableDebug bool
	var err error
	if srv := s.agent.Server(); srv != nil {
		aclObj, err = srv.ResolveToken(secret)
		enableDebug = srv.GetConfig().EnableDebug
	} else {
		// Not a Server; use the Client for token resolution
		aclObj, err = s.agent.Client().ResolveToken(secret)
		enableDebug = s.agent.Client().GetConfig().EnableDebug
	}
	if err != nil {
		return nil, err
	}

	if (aclObj != nil && !aclObj.AllowAgentRead()) ||
		(aclObj == nil && !enableDebug) {
		return nil, structs.ErrPermissionDenied
	}

	serverID := req.URL.Query().Get("server_id")
	nodeID := req.URL.Query().Get("node_id")

	if serverID != "" && nodeID != "" {
		return nil, CodedError(400, "Can only forward to either client node or server")
	}

	// If no other node is specified, return our local host's data
	if serverID == "" && nodeID == "" {
		data, err := host.MakeHostData()
		if err != nil {
			return nil, CodedError(500, err.Error())
		}
		return data, nil
	}

	args := &structs.HostDataRequest{
		ServerID: serverID,
		NodeID:   nodeID,
	}

	s.parse(resp, req, &args.QueryOptions.Region, &args.QueryOptions)

	var reply structs.HostDataResponse
	var rpcErr error

	// serverID is set, so forward to that server
	if serverID != "" {
		rpcErr = s.agent.Server().RPC("Agent.Host", &args, &reply)
		return reply, rpcErr
	}

	// Make the RPC. The RPC endpoint actually forwards the request to the correct
	// agent, but we need to use the correct RPC interface.
	localClient, remoteClient, localServer := s.rpcHandlerForNode(nodeID)

	if localClient {
		rpcErr = s.agent.Client().ClientRPC("Agent.Host", &args, &reply)
	} else if remoteClient {
		rpcErr = s.agent.Client().RPC("Agent.Host", &args, &reply)
	} else if localServer {
		rpcErr = s.agent.Server().RPC("Agent.Host", &args, &reply)
	} else {
		rpcErr = fmt.Errorf("node not found: %s", nodeID)
	}

	return reply, rpcErr
}

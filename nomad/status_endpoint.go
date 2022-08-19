package nomad

import (
	"errors"
	"fmt"
	"strconv"

	log "github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/consul/autopilot"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/nomad/structs"
)

// Status endpoint is used to check on server status
type Status struct {
	srv    *Server
	logger log.Logger

	// ctx provides context regarding the underlying connection
	rpcCtx *RPCContext
}

func (s *Status) checkRateLimit(forPolicy, rateLimitToken string) error {
	if err := s.srv.CheckRateLimit("Status", forPolicy, rateLimitToken, s.rpcCtx); err != nil {
		return err
	}
	return nil
}

func (s *Status) rpcNodeID() string {
	if s.rpcCtx != nil {
		return s.rpcCtx.NodeID
	}
	return ""
}

// Ping is used to just check for connectivity
func (s *Status) Ping(args struct{}, reply *struct{}) error {
	if err := s.checkRateLimit(acl.PolicyRead, s.rpcNodeID()); err != nil {
		return err
	}

	return nil
}

// Leader is used to get the address of the leader
func (s *Status) Leader(args *structs.GenericRequest, reply *string) error {
	if err := s.checkRateLimit(acl.PolicyRead, args.AuthToken); err != nil {
		return err
	}

	if args.Region == "" {
		args.Region = s.srv.config.Region
	}
	if done, err := s.srv.forward("Status.Leader", args, args, reply); done {
		return err
	}

	leader := string(s.srv.raft.Leader())
	if leader != "" {
		*reply = leader
	} else {
		*reply = ""
	}
	return nil
}

// Peers is used to get all the Raft peers
func (s *Status) Peers(args *structs.GenericRequest, reply *[]string) error {
	if err := s.checkRateLimit(acl.PolicyList, args.AuthToken); err != nil {
		return err
	}

	if args.Region == "" {
		args.Region = s.srv.config.Region
	}
	if done, err := s.srv.forward("Status.Peers", args, args, reply); done {
		return err
	}

	future := s.srv.raft.GetConfiguration()
	if err := future.Error(); err != nil {
		return err
	}

	for _, server := range future.Configuration().Servers {
		*reply = append(*reply, string(server.Address))
	}
	return nil
}

// Members return the list of servers in a cluster that a particular server is
// aware of
func (s *Status) Members(args *structs.GenericRequest, reply *structs.ServerMembersResponse) error {
	if err := s.checkRateLimit(acl.PolicyList, args.AuthToken); err != nil {
		return err
	}

	// Check node read permissions
	if aclObj, err := s.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowNodeRead() {
		return structs.ErrPermissionDenied
	}

	serfMembers := s.srv.Members()
	members := make([]*structs.ServerMember, len(serfMembers))
	for i, mem := range serfMembers {
		members[i] = &structs.ServerMember{
			Name:        mem.Name,
			Addr:        mem.Addr,
			Port:        mem.Port,
			Tags:        mem.Tags,
			Status:      mem.Status.String(),
			ProtocolMin: mem.ProtocolMin,
			ProtocolMax: mem.ProtocolMax,
			ProtocolCur: mem.ProtocolCur,
			DelegateMin: mem.DelegateMin,
			DelegateMax: mem.DelegateMax,
			DelegateCur: mem.DelegateCur,
		}
	}
	*reply = structs.ServerMembersResponse{
		ServerName:   s.srv.config.NodeName,
		ServerRegion: s.srv.config.Region,
		ServerDC:     s.srv.config.Datacenter,
		Members:      members,
	}
	return nil
}

// RaftStats is used by Autopilot to query the raft stats of the local server.
func (s *Status) RaftStats(args struct{}, reply *autopilot.ServerStats) error {
	if err := s.checkRateLimit(acl.PolicyRead, s.rpcNodeID()); err != nil {
		return err
	}

	stats := s.srv.raft.Stats()

	var err error
	reply.LastContact = stats["last_contact"]
	reply.LastIndex, err = strconv.ParseUint(stats["last_log_index"], 10, 64)
	if err != nil {
		return fmt.Errorf("error parsing server's last_log_index value: %s", err)
	}
	reply.LastTerm, err = strconv.ParseUint(stats["last_log_term"], 10, 64)
	if err != nil {
		return fmt.Errorf("error parsing server's last_log_term value: %s", err)
	}

	return nil
}

// HasNodeConn returns whether the server has a connection to the requested
// Node.
func (s *Status) HasNodeConn(args *structs.NodeSpecificRequest, reply *structs.NodeConnQueryResponse) error {
	if err := s.checkRateLimit(acl.PolicyRead, s.rpcNodeID()); err != nil {
		return err
	}

	// Validate the args
	if args.NodeID == "" {
		return errors.New("Must provide the NodeID")
	}

	state, ok := s.srv.getNodeConn(args.NodeID)
	if ok {
		reply.Connected = true
		reply.Established = state.Established
	}

	return nil
}

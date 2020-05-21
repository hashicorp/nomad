package nomad

import (
	"fmt"
	"io"
	"net"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-msgpack/codec"

	"github.com/hashicorp/consul/agent/consul/autopilot"
	"github.com/hashicorp/nomad/helper/snapshot"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
)

// Operator endpoint is used to perform low-level operator tasks for Nomad.
type Operator struct {
	srv    *Server
	logger log.Logger
}

func (op *Operator) register() {
	op.srv.streamingRpcs.Register("Operator.SnapshotSave", op.snapshotSave)
}

// RaftGetConfiguration is used to retrieve the current Raft configuration.
func (op *Operator) RaftGetConfiguration(args *structs.GenericRequest, reply *structs.RaftConfigurationResponse) error {
	if done, err := op.srv.forward("Operator.RaftGetConfiguration", args, args, reply); done {
		return err
	}

	// Check management permissions
	if aclObj, err := op.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.IsManagement() {
		return structs.ErrPermissionDenied
	}

	// We can't fetch the leader and the configuration atomically with
	// the current Raft API.
	future := op.srv.raft.GetConfiguration()
	if err := future.Error(); err != nil {
		return err
	}

	// Index the Nomad information about the servers.
	serverMap := make(map[raft.ServerAddress]serf.Member)
	for _, member := range op.srv.serf.Members() {
		valid, parts := isNomadServer(member)
		if !valid {
			continue
		}

		addr := (&net.TCPAddr{IP: member.Addr, Port: parts.Port}).String()
		serverMap[raft.ServerAddress(addr)] = member
	}

	// Fill out the reply.
	leader := op.srv.raft.Leader()
	reply.Index = future.Index()
	for _, server := range future.Configuration().Servers {
		node := "(unknown)"
		raftProtocolVersion := "unknown"
		if member, ok := serverMap[server.Address]; ok {
			node = member.Name
			if raftVsn, ok := member.Tags["raft_vsn"]; ok {
				raftProtocolVersion = raftVsn
			}
		}

		entry := &structs.RaftServer{
			ID:           server.ID,
			Node:         node,
			Address:      server.Address,
			Leader:       server.Address == leader,
			Voter:        server.Suffrage == raft.Voter,
			RaftProtocol: raftProtocolVersion,
		}
		reply.Servers = append(reply.Servers, entry)
	}
	return nil
}

// RaftRemovePeerByAddress is used to kick a stale peer (one that it in the Raft
// quorum but no longer known to Serf or the catalog) by address in the form of
// "IP:port". The reply argument is not used, but it required to fulfill the RPC
// interface.
func (op *Operator) RaftRemovePeerByAddress(args *structs.RaftPeerByAddressRequest, reply *struct{}) error {
	if done, err := op.srv.forward("Operator.RaftRemovePeerByAddress", args, args, reply); done {
		return err
	}

	// Check management permissions
	if aclObj, err := op.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.IsManagement() {
		return structs.ErrPermissionDenied
	}

	// Since this is an operation designed for humans to use, we will return
	// an error if the supplied address isn't among the peers since it's
	// likely they screwed up.
	{
		future := op.srv.raft.GetConfiguration()
		if err := future.Error(); err != nil {
			return err
		}
		for _, s := range future.Configuration().Servers {
			if s.Address == args.Address {
				goto REMOVE
			}
		}
		return fmt.Errorf("address %q was not found in the Raft configuration",
			args.Address)
	}

REMOVE:
	// The Raft library itself will prevent various forms of foot-shooting,
	// like making a configuration with no voters. Some consideration was
	// given here to adding more checks, but it was decided to make this as
	// low-level and direct as possible. We've got ACL coverage to lock this
	// down, and if you are an operator, it's assumed you know what you are
	// doing if you are calling this. If you remove a peer that's known to
	// Serf, for example, it will come back when the leader does a reconcile
	// pass.
	future := op.srv.raft.RemovePeer(args.Address)
	if err := future.Error(); err != nil {
		op.logger.Warn("failed to remove Raft peer", "peer", args.Address, "error", err)
		return err
	}

	op.logger.Warn("removed Raft peer", "peer", args.Address)
	return nil
}

// RaftRemovePeerByID is used to kick a stale peer (one that is in the Raft
// quorum but no longer known to Serf or the catalog) by address in the form of
// "IP:port". The reply argument is not used, but is required to fulfill the RPC
// interface.
func (op *Operator) RaftRemovePeerByID(args *structs.RaftPeerByIDRequest, reply *struct{}) error {
	if done, err := op.srv.forward("Operator.RaftRemovePeerByID", args, args, reply); done {
		return err
	}

	// Check management permissions
	if aclObj, err := op.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.IsManagement() {
		return structs.ErrPermissionDenied
	}

	// Since this is an operation designed for humans to use, we will return
	// an error if the supplied id isn't among the peers since it's
	// likely they screwed up.
	var address raft.ServerAddress
	{
		future := op.srv.raft.GetConfiguration()
		if err := future.Error(); err != nil {
			return err
		}
		for _, s := range future.Configuration().Servers {
			if s.ID == args.ID {
				address = s.Address
				goto REMOVE
			}
		}
		return fmt.Errorf("id %q was not found in the Raft configuration",
			args.ID)
	}

REMOVE:
	// The Raft library itself will prevent various forms of foot-shooting,
	// like making a configuration with no voters. Some consideration was
	// given here to adding more checks, but it was decided to make this as
	// low-level and direct as possible. We've got ACL coverage to lock this
	// down, and if you are an operator, it's assumed you know what you are
	// doing if you are calling this. If you remove a peer that's known to
	// Serf, for example, it will come back when the leader does a reconcile
	// pass.
	minRaftProtocol, err := op.srv.autopilot.MinRaftProtocol()
	if err != nil {
		return err
	}

	var future raft.Future
	if minRaftProtocol >= 2 {
		future = op.srv.raft.RemoveServer(args.ID, 0, 0)
	} else {
		future = op.srv.raft.RemovePeer(address)
	}
	if err := future.Error(); err != nil {
		op.logger.Warn("failed to remove Raft peer", "peer_id", args.ID, "error", err)
		return err
	}

	op.logger.Warn("removed Raft peer", "peer_id", args.ID)
	return nil
}

// AutopilotGetConfiguration is used to retrieve the current Autopilot configuration.
func (op *Operator) AutopilotGetConfiguration(args *structs.GenericRequest, reply *structs.AutopilotConfig) error {
	if done, err := op.srv.forward("Operator.AutopilotGetConfiguration", args, args, reply); done {
		return err
	}

	// This action requires operator read access.
	rule, err := op.srv.ResolveToken(args.AuthToken)
	if err != nil {
		return err
	}
	if rule != nil && !rule.AllowOperatorRead() {
		return structs.ErrPermissionDenied
	}

	state := op.srv.fsm.State()
	_, config, err := state.AutopilotConfig()
	if err != nil {
		return err
	}
	if config == nil {
		return fmt.Errorf("autopilot config not initialized yet")
	}

	*reply = *config

	return nil
}

// AutopilotSetConfiguration is used to set the current Autopilot configuration.
func (op *Operator) AutopilotSetConfiguration(args *structs.AutopilotSetConfigRequest, reply *bool) error {
	if done, err := op.srv.forward("Operator.AutopilotSetConfiguration", args, args, reply); done {
		return err
	}

	// This action requires operator write access.
	rule, err := op.srv.ResolveToken(args.AuthToken)
	if err != nil {
		return err
	}
	if rule != nil && !rule.AllowOperatorWrite() {
		return structs.ErrPermissionDenied
	}

	// All servers should be at or above 0.8.0 to apply this operatation
	if !ServersMeetMinimumVersion(op.srv.Members(), minAutopilotVersion, false) {
		return fmt.Errorf("All servers should be running version %v to update autopilot config", minAutopilotVersion)
	}

	// Apply the update
	resp, _, err := op.srv.raftApply(structs.AutopilotRequestType, args)
	if err != nil {
		op.logger.Error("failed applying AutoPilot configuration", "error", err)
		return err
	}
	if respErr, ok := resp.(error); ok {
		return respErr
	}

	// Check if the return type is a bool.
	if respBool, ok := resp.(bool); ok {
		*reply = respBool
	}
	return nil
}

// ServerHealth is used to get the current health of the servers.
func (op *Operator) ServerHealth(args *structs.GenericRequest, reply *autopilot.OperatorHealthReply) error {
	// This must be sent to the leader, so we fix the args since we are
	// re-using a structure where we don't support all the options.
	args.AllowStale = false
	if done, err := op.srv.forward("Operator.ServerHealth", args, args, reply); done {
		return err
	}

	// This action requires operator read access.
	rule, err := op.srv.ResolveToken(args.AuthToken)
	if err != nil {
		return err
	}
	if rule != nil && !rule.AllowOperatorRead() {
		return structs.ErrPermissionDenied
	}

	// Exit early if the min Raft version is too low
	minRaftProtocol, err := op.srv.autopilot.MinRaftProtocol()
	if err != nil {
		return fmt.Errorf("error getting server raft protocol versions: %s", err)
	}
	if minRaftProtocol < 3 {
		return fmt.Errorf("all servers must have raft_protocol set to 3 or higher to use this endpoint")
	}

	*reply = op.srv.autopilot.GetClusterHealth()

	return nil
}

// SchedulerSetConfiguration is used to set the current Scheduler configuration.
func (op *Operator) SchedulerSetConfiguration(args *structs.SchedulerSetConfigRequest, reply *structs.SchedulerSetConfigurationResponse) error {
	if done, err := op.srv.forward("Operator.SchedulerSetConfiguration", args, args, reply); done {
		return err
	}

	// This action requires operator write access.
	rule, err := op.srv.ResolveToken(args.AuthToken)
	if err != nil {
		return err
	} else if rule != nil && !rule.AllowOperatorWrite() {
		return structs.ErrPermissionDenied
	}

	// All servers should be at or above 0.9.0 to apply this operatation
	if !ServersMeetMinimumVersion(op.srv.Members(), minSchedulerConfigVersion, false) {
		return fmt.Errorf("All servers should be running version %v to update scheduler config", minSchedulerConfigVersion)
	}
	// Apply the update
	resp, index, err := op.srv.raftApply(structs.SchedulerConfigRequestType, args)
	if err != nil {
		op.logger.Error("failed applying Scheduler configuration", "error", err)
		return err
	} else if respErr, ok := resp.(error); ok {
		return respErr
	}

	// Check if the return type is a bool
	// Only applies to CAS requests
	if respBool, ok := resp.(bool); ok {
		reply.Updated = respBool
	}
	reply.Index = index
	return nil
}

// SchedulerGetConfiguration is used to retrieve the current Scheduler configuration.
func (op *Operator) SchedulerGetConfiguration(args *structs.GenericRequest, reply *structs.SchedulerConfigurationResponse) error {
	if done, err := op.srv.forward("Operator.SchedulerGetConfiguration", args, args, reply); done {
		return err
	}

	// This action requires operator read access.
	rule, err := op.srv.ResolveToken(args.AuthToken)
	if err != nil {
		return err
	} else if rule != nil && !rule.AllowOperatorRead() {
		return structs.ErrPermissionDenied
	}

	state := op.srv.fsm.State()
	index, config, err := state.SchedulerConfig()

	if err != nil {
		return err
	} else if config == nil {
		return fmt.Errorf("scheduler config not initialized yet")
	}

	reply.SchedulerConfig = config
	reply.QueryMeta.Index = index
	op.srv.setQueryMeta(&reply.QueryMeta)

	return nil
}

func (op *Operator) forwardStreamingRPC(region string, method string, args interface{}, in io.ReadWriteCloser) error {
	server, err := op.srv.findRegionServer(region)
	if err != nil {
		return err
	}

	return op.forwardStreamingRPCToServer(server, method, args, in)
}

func (op *Operator) forwardStreamingRPCToServer(server *serverParts, method string, args interface{}, in io.ReadWriteCloser) error {
	srvConn, err := op.srv.streamingRpc(server, method)
	if err != nil {
		return err
	}
	defer srvConn.Close()

	outEncoder := codec.NewEncoder(srvConn, structs.MsgpackHandle)
	if err := outEncoder.Encode(args); err != nil {
		return err
	}

	structs.Bridge(in, srvConn)
	return nil
}

func (op *Operator) snapshotSave(conn io.ReadWriteCloser) {
	defer conn.Close()

	var args structs.SnapshotSaveRequest
	var reply structs.SnapshotSaveResponse
	decoder := codec.NewDecoder(conn, structs.MsgpackHandle)
	encoder := codec.NewEncoder(conn, structs.MsgpackHandle)

	handleFailure := func(code int, err error) {
		encoder.Encode(&structs.SnapshotSaveResponse{
			ErrorCode: code,
			ErrorMsg:  err.Error(),
		})
	}

	if err := decoder.Decode(&args); err != nil {
		handleFailure(500, err)
		return
	}

	// Forward to appropriate region
	if args.Region != op.srv.Region() {
		err := op.forwardStreamingRPC(args.Region, "Operator.SnapshotSave", args, conn)
		if err != nil {
			handleFailure(500, err)
		}
		return
	}

	// forward to leader
	if !args.AllowStale {
		remoteServer, err := op.srv.getLeaderForRPC()
		if err != nil {
			handleFailure(500, err)
			return
		}
		if remoteServer != nil {
			err := op.forwardStreamingRPCToServer(remoteServer, "Operator.SnapshotSave", args, conn)
			if err != nil {
				handleFailure(500, err)
			}
			return

		}
	}

	// Check agent permissions
	if aclObj, err := op.srv.ResolveToken(args.AuthToken); err != nil {
		code := 500
		if err == structs.ErrTokenNotFound {
			code = 400
		}
		handleFailure(code, err)
		return
	} else if aclObj != nil && !aclObj.IsManagement() {
		handleFailure(403, structs.ErrPermissionDenied)
		return
	}

	op.srv.setQueryMeta(&reply.QueryMeta)

	// Take the snapshot and capture the index.
	snap, err := snapshot.New(op.logger, op.srv.raft)
	reply.SnapshotChecksum = snap.Checksum()
	reply.Index = snap.Index()
	if err != nil {
		handleFailure(500, err)
		return
	}
	defer snap.Close()

	enc := codec.NewEncoder(conn, structs.MsgpackHandle)
	if err := enc.Encode(&reply); err != nil {
		handleFailure(500, fmt.Errorf("failed to encode response: %v", err))
		return
	}
	if snap != nil {
		if _, err := io.Copy(conn, snap); err != nil {
			handleFailure(500, fmt.Errorf("failed to stream snapshot: %v", err))
		}
	}
}

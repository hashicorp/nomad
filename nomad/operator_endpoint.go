// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-msgpack/codec"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"

	"github.com/hashicorp/nomad/api"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/snapshot"
	"github.com/hashicorp/nomad/nomad/structs"
)

// Operator endpoint is used to perform low-level operator tasks for Nomad.
type Operator struct {
	srv    *Server
	ctx    *RPCContext
	logger hclog.Logger
}

func NewOperatorEndpoint(srv *Server, ctx *RPCContext) *Operator {
	return &Operator{srv: srv, ctx: ctx, logger: srv.logger.Named("operator")}
}

func (op *Operator) register() {
	op.srv.streamingRpcs.Register("Operator.SnapshotSave", op.snapshotSave)
	op.srv.streamingRpcs.Register("Operator.SnapshotRestore", op.snapshotRestore)
}

// RaftGetConfiguration is used to retrieve the current Raft configuration.
func (op *Operator) RaftGetConfiguration(args *structs.GenericRequest, reply *structs.RaftConfigurationResponse) error {

	authErr := op.srv.Authenticate(op.ctx, args)
	if done, err := op.srv.forward("Operator.RaftGetConfiguration", args, args, reply); done {
		return err
	}
	op.srv.MeasureRPCRate("operator", structs.RateMetricRead, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}

	// Check management permissions
	if aclObj, err := op.srv.ResolveACL(args); err != nil {
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
	leader, _ := op.srv.raft.LeaderWithID()
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

	authErr := op.srv.Authenticate(op.ctx, args)
	if done, err := op.srv.forward("Operator.RaftRemovePeerByAddress", args, args, reply); done {
		return err
	}
	op.srv.MeasureRPCRate("operator", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}

	// Check management permissions
	if aclObj, err := op.srv.ResolveACL(args); err != nil {
		return err
	} else if aclObj != nil && !aclObj.IsManagement() {
		return structs.ErrPermissionDenied
	}

	// Since this is an operation designed for humans to use, we will return
	// an error if the supplied address isn't among the peers since it's
	// likely a mistake.
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

	authErr := op.srv.Authenticate(op.ctx, args)
	if done, err := op.srv.forward("Operator.RaftRemovePeerByID", args, args, reply); done {
		return err
	}
	op.srv.MeasureRPCRate("operator", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}

	// Check management permissions
	if aclObj, err := op.srv.ResolveACL(args); err != nil {
		return err
	} else if aclObj != nil && !aclObj.IsManagement() {
		return structs.ErrPermissionDenied
	}

	// Since this is an operation designed for humans to use, we will return
	// an error if the supplied id isn't among the peers since it's
	// likely a mistake.
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
	minRaftProtocol, err := op.srv.MinRaftProtocol()
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

// TransferLeadershipToServerID is used to transfer leadership away from the
// current leader to a specific target peer. This can help prevent leadership
// flapping during a rolling upgrade by allowing the cluster operator to target
// an already upgraded node before upgrading the remainder of the cluster.
func (op *Operator) TransferLeadershipToPeer(req *structs.RaftPeerRequest, reply *api.LeadershipTransferResponse) error {
	reply.To.Address, reply.To.ID = string(req.Address), string(req.ID)
	tgtAddr, tgtID := req.Address, req.ID

	authErr := op.srv.Authenticate(op.ctx, req)

	if done, err := op.srv.forward("Operator.TransferLeadershipToPeer", req, req, reply); done {
		reply.Err = err
		return reply.Err
	}
	op.srv.MeasureRPCRate("operator", structs.RateMetricWrite, req)
	if authErr != nil {
		reply.Err = structs.ErrPermissionDenied
		return structs.ErrPermissionDenied
	}

	// Check ACL permissions
	if aclObj, err := op.srv.ResolveACL(req); err != nil {
		return err
	} else if aclObj != nil && !aclObj.IsManagement() {
		reply.Err = structs.ErrPermissionDenied
		return structs.ErrPermissionDenied
	}

	// Technically, this code will be running on the leader becuase of the RPC
	// forwarding, but a leadership change could happen at any moment while we're
	// running. We need the leader's raft info to populate the response struct
	// anyway, so we have a chance to check again here
	lAddr, lID := op.srv.raft.LeaderWithID()
	reply.From.Address, reply.From.ID = string(lAddr), string(lID)

	// If the leader information comes back empty, that signals that there is
	// currently no leader.
	if lAddr == "" || lID == "" {
		reply.Err = structs.ErrNoLeader
		return structs.NewErrRPCCoded(http.StatusServiceUnavailable, structs.ErrNoLeader.Error())
	}

	// while this is a somewhat more expensive test than later ones, if this
	// test fails, they will _never_ be able to do a transfer. We do this after
	// ACL checks though, so as to not leak cluster info to unvalidated users.
	minRaftProtocol, err := op.srv.MinRaftProtocol()
	if err != nil {
		reply.Err = err
		return err
	}

	// TransferLeadership is not supported until Raft protocol v3 or greater.
	if minRaftProtocol < 3 {
		op.logger.Warn("unsupported minimum common raft protocol version", "required", "3", "current", minRaftProtocol)
		reply.Err = errors.New("unsupported minimum common raft protocol version")
		return structs.NewErrRPCCoded(http.StatusBadRequest, reply.Err.Error())
	}

	var kind, testedVal string

	// The request must provide either an ID or an Address, this lets us validate
	// the request
	req.Validate()
	switch {
	case req.ID != "":
		kind, testedVal = "id", string(req.ID)
	case req.Address != "":
		kind, testedVal = "address", string(req.Address)
	default:
		reply.Err = errors.New("must provide peer id or address")
		return structs.NewErrRPCCoded(http.StatusBadRequest, reply.Err.Error())
	}

	// Fetching lAddr and lID again close to use so we can
	if lAddr, lID := op.srv.raft.LeaderWithID(); lAddr == "" || lID == "" ||
		(tgtID == lID && tgtAddr == lAddr) {

		// If the leader info is empty, return a ErrNoLeader
		if lAddr == "" || lID == "" {
			reply.Err = structs.ErrNoLeader
			return structs.NewErrRPCCoded(http.StatusServiceUnavailable, structs.ErrNoLeader.Error())
		}

		// Otherwise, this is a no-op, respond accordingly.
		reply.From.Address, reply.From.ID = string(lAddr), string(lID)
		op.logger.Debug("leadership transfer to current leader is a no-op")
		reply.Noop = true
		return nil
	}

	// Get the raft configuration
	future := op.srv.raft.GetConfiguration()
	if err := future.Error(); err != nil {
		reply.Err = err
		return err
	}

	// Since this is an operation designed for humans to use, we will return
	// an error if the supplied ID or address isn't among the peers since it's
	// likely a mistake.
	var found bool
	for _, s := range future.Configuration().Servers {
		if s.ID == req.ID || s.Address == req.Address {
			tgtID = s.ID
			tgtAddr = s.Address
			found = true
		}
	}

	if !found {
		reply.Err = fmt.Errorf("%s %q was not found in the Raft configuration",
			kind, testedVal)
		return structs.NewErrRPCCoded(http.StatusBadRequest, reply.Err.Error())
	}

	log := op.logger.With("to_peer_id", tgtID, "to_peer_addr", tgtAddr, "from_peer_id", lID, "from_peer_addr", lAddr)
	if err = op.srv.leadershipTransferToServer(tgtID, tgtAddr); err != nil {
		reply.Err = err
		log.Error("failed transferring leadership", "error", reply.Err.Error())
		return err
	}

	log.Info("transferred leadership")
	return nil
}

// AutopilotGetConfiguration is used to retrieve the current Autopilot configuration.
func (op *Operator) AutopilotGetConfiguration(args *structs.GenericRequest, reply *structs.AutopilotConfig) error {

	authErr := op.srv.Authenticate(op.ctx, args)
	if done, err := op.srv.forward("Operator.AutopilotGetConfiguration", args, args, reply); done {
		return err
	}
	op.srv.MeasureRPCRate("operator", structs.RateMetricRead, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}

	// This action requires operator read access.
	rule, err := op.srv.ResolveACL(args)
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

	authErr := op.srv.Authenticate(op.ctx, args)
	if done, err := op.srv.forward("Operator.AutopilotSetConfiguration", args, args, reply); done {
		return err
	}
	op.srv.MeasureRPCRate("operator", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}

	// This action requires operator write access.
	rule, err := op.srv.ResolveACL(args)
	if err != nil {
		return err
	}
	if rule != nil && !rule.AllowOperatorWrite() {
		return structs.ErrPermissionDenied
	}

	// All servers should be at or above 0.8.0 to apply this operatation
	if !ServersMeetMinimumVersion(op.srv.Members(), op.srv.Region(), minAutopilotVersion, false) {
		return fmt.Errorf("All servers should be running version %v to update autopilot config", minAutopilotVersion)
	}

	// Apply the update
	resp, _, err := op.srv.raftApply(structs.AutopilotRequestType, args)
	if err != nil {
		op.logger.Error("failed applying AutoPilot configuration", "error", err)
		return err
	}

	// Check if the return type is a bool.
	if respBool, ok := resp.(bool); ok {
		*reply = respBool
	}
	return nil
}

// ServerHealth is used to get the current health of the servers.
func (op *Operator) ServerHealth(args *structs.GenericRequest, reply *structs.OperatorHealthReply) error {

	authErr := op.srv.Authenticate(op.ctx, args)
	// This must be sent to the leader, so we fix the args since we are
	// re-using a structure where we don't support all the options.
	args.AllowStale = false
	if done, err := op.srv.forward("Operator.ServerHealth", args, args, reply); done {
		return err
	}
	op.srv.MeasureRPCRate("operator", structs.RateMetricRead, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}

	// This action requires operator read access.
	rule, err := op.srv.ResolveACL(args)
	if err != nil {
		return err
	}
	if rule != nil && !rule.AllowOperatorRead() {
		return structs.ErrPermissionDenied
	}

	// Exit early if the min Raft version is too low
	minRaftProtocol, err := op.srv.MinRaftProtocol()
	if err != nil {
		return fmt.Errorf("error getting server raft protocol versions: %s", err)
	}
	if minRaftProtocol < 3 {
		return fmt.Errorf("all servers must have raft_protocol set to 3 or higher to use this endpoint")
	}

	*reply = *op.srv.GetClusterHealth()

	return nil
}

// SchedulerSetConfiguration is used to set the current Scheduler configuration.
func (op *Operator) SchedulerSetConfiguration(args *structs.SchedulerSetConfigRequest, reply *structs.SchedulerSetConfigurationResponse) error {

	authErr := op.srv.Authenticate(op.ctx, args)
	if done, err := op.srv.forward("Operator.SchedulerSetConfiguration", args, args, reply); done {
		return err
	}
	op.srv.MeasureRPCRate("operator", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}

	// This action requires operator write access.
	rule, err := op.srv.ResolveACL(args)
	if err != nil {
		return err
	} else if rule != nil && !rule.AllowOperatorWrite() {
		return structs.ErrPermissionDenied
	}

	// All servers should be at or above 0.9.0 to apply this operation
	if !ServersMeetMinimumVersion(op.srv.Members(), op.srv.Region(), minSchedulerConfigVersion, false) {
		return fmt.Errorf("All servers should be running version %v to update scheduler config", minSchedulerConfigVersion)
	}

	// Apply the update
	resp, index, err := op.srv.raftApply(structs.SchedulerConfigRequestType, args)
	if err != nil {
		op.logger.Error("failed applying Scheduler configuration", "error", err)
		return err
	}

	//  If CAS request, raft returns a boolean indicating if the update was applied.
	// Otherwise, assume success
	reply.Updated = true
	if respBool, ok := resp.(bool); ok {
		reply.Updated = respBool
	}

	reply.Index = index

	// If we updated the configuration, handle any required state changes within
	// the eval broker and blocked evals processes. The state change and
	// restore functions have protections around leadership transitions and
	// restoring into non-running brokers.
	if reply.Updated {
		if op.srv.handleEvalBrokerStateChange(&args.Config) {
			return op.srv.restoreEvals()
		}
	}

	return nil
}

// SchedulerGetConfiguration is used to retrieve the current Scheduler configuration.
func (op *Operator) SchedulerGetConfiguration(args *structs.GenericRequest, reply *structs.SchedulerConfigurationResponse) error {

	authErr := op.srv.Authenticate(op.ctx, args)
	if done, err := op.srv.forward("Operator.SchedulerGetConfiguration", args, args, reply); done {
		return err
	}
	op.srv.MeasureRPCRate("operator", structs.RateMetricRead, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}

	// This action requires operator read access.
	rule, err := op.srv.ResolveACL(args)
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

	authErr := op.srv.Authenticate(nil, &args)

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

	op.srv.MeasureRPCRate("operator", structs.RateMetricWrite, &args)
	if authErr != nil {
		handleFailure(403, structs.ErrPermissionDenied)
	}

	// Check agent permissions
	if aclObj, err := op.srv.ResolveACL(&args); err != nil {
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
	snap, err := snapshot.New(op.logger.Named("snapshot"), op.srv.raft)
	reply.SnapshotChecksum = snap.Checksum()
	reply.Index = snap.Index()
	if err != nil {
		handleFailure(500, err)
		return
	}
	defer snap.Close()

	if err := encoder.Encode(&reply); err != nil {
		handleFailure(500, fmt.Errorf("failed to encode response: %v", err))
		return
	}
	if snap != nil {
		if _, err := io.Copy(conn, snap); err != nil {
			handleFailure(500, fmt.Errorf("failed to stream snapshot: %v", err))
		}
	}
}

func (op *Operator) snapshotRestore(conn io.ReadWriteCloser) {
	defer conn.Close()

	var args structs.SnapshotRestoreRequest
	var reply structs.SnapshotRestoreResponse
	decoder := codec.NewDecoder(conn, structs.MsgpackHandle)
	encoder := codec.NewEncoder(conn, structs.MsgpackHandle)

	handleFailure := func(code int, err error) {
		encoder.Encode(&structs.SnapshotRestoreResponse{
			ErrorCode: code,
			ErrorMsg:  err.Error(),
		})
	}

	if err := decoder.Decode(&args); err != nil {
		handleFailure(500, err)
		return
	}

	authErr := op.srv.Authenticate(nil, &args)

	// Forward to appropriate region
	if args.Region != op.srv.Region() {
		err := op.forwardStreamingRPC(args.Region, "Operator.SnapshotRestore", args, conn)
		if err != nil {
			handleFailure(500, err)
		}
		return
	}

	// forward to leader
	remoteServer, err := op.srv.getLeaderForRPC()
	if err != nil {
		handleFailure(500, err)
		return
	}
	if remoteServer != nil {
		err := op.forwardStreamingRPCToServer(remoteServer, "Operator.SnapshotRestore", args, conn)
		if err != nil {
			handleFailure(500, err)
		}
		return

	}

	op.srv.MeasureRPCRate("operator", structs.RateMetricWrite, &args)
	if authErr != nil {
		handleFailure(403, structs.ErrPermissionDenied)
	}

	// Check agent permissions
	if aclObj, err := op.srv.ResolveACL(&args); err != nil {
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

	reader, errCh := decodeStreamOutput(decoder)

	err = snapshot.Restore(op.logger.Named("snapshot"), reader, op.srv.raft)
	if err != nil {
		handleFailure(500, fmt.Errorf("failed to restore from snapshot: %v", err))
		return
	}

	err = <-errCh
	if err != nil {
		handleFailure(400, fmt.Errorf("failed to read stream: %v", err))
		return
	}

	// This'll be used for feedback from the leader loop.
	timeoutCh := time.After(time.Minute)

	lerrCh := make(chan error, 1)

	select {
	// Reassert leader actions and update all leader related state
	// with new state store content.
	case op.srv.reassertLeaderCh <- lerrCh:

	// We might have lost leadership while waiting to kick the loop.
	case <-timeoutCh:
		handleFailure(500, fmt.Errorf("timed out waiting to re-run leader actions"))

	// Make sure we don't get stuck during shutdown
	case <-op.srv.shutdownCh:
	}

	select {
	// Wait for the leader loop to finish up.
	case err := <-lerrCh:
		if err != nil {
			handleFailure(500, err)
			return
		}

	// We might have lost leadership while the loop was doing its
	// thing.
	case <-timeoutCh:
		handleFailure(500, fmt.Errorf("timed out waiting for re-run of leader actions"))

	// Make sure we don't get stuck during shutdown
	case <-op.srv.shutdownCh:
	}

	reply.Index, _ = op.srv.State().LatestIndex()
	op.srv.setQueryMeta(&reply.QueryMeta)
	encoder.Encode(reply)
}

func decodeStreamOutput(decoder *codec.Decoder) (io.Reader, <-chan error) {
	pr, pw := io.Pipe()
	errCh := make(chan error, 1)

	go func() {
		defer close(errCh)

		for {
			var wrapper cstructs.StreamErrWrapper

			err := decoder.Decode(&wrapper)
			if err != nil {
				pw.CloseWithError(fmt.Errorf("failed to decode input: %v", err))
				errCh <- err
				return
			}

			if len(wrapper.Payload) != 0 {
				_, err = pw.Write(wrapper.Payload)
				if err != nil {
					pw.CloseWithError(err)
					errCh <- err
					return
				}
			}

			if errW := wrapper.Error; errW != nil {
				if errW.Message == io.EOF.Error() {
					pw.CloseWithError(io.EOF)
				} else {
					pw.CloseWithError(errors.New(errW.Message))
				}
				return
			}
		}
	}()

	return pr, errCh
}

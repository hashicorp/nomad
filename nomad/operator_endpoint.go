// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-msgpack/codec"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"

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

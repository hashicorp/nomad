// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-multierror"
	vapi "github.com/hashicorp/vault/api"
	"golang.org/x/sync/errgroup"

	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/state/paginator"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/raft"
)

const (
	// batchUpdateInterval is how long we wait to batch updates
	batchUpdateInterval = 50 * time.Millisecond

	// maxParallelRequestsPerDerive  is the maximum number of parallel Vault
	// create token requests that may be outstanding per derive request
	maxParallelRequestsPerDerive = 16

	// NodeDrainEvents are the various drain messages
	NodeDrainEventDrainSet      = "Node drain strategy set"
	NodeDrainEventDrainDisabled = "Node drain disabled"
	NodeDrainEventDrainUpdated  = "Node drain strategy updated"

	// NodeEligibilityEventEligible is used when the nodes eligiblity is marked
	// eligible
	NodeEligibilityEventEligible = "Node marked as eligible for scheduling"

	// NodeEligibilityEventIneligible is used when the nodes eligiblity is marked
	// ineligible
	NodeEligibilityEventIneligible = "Node marked as ineligible for scheduling"

	// NodeHeartbeatEventReregistered is the message used when the node becomes
	// reregistered by the heartbeat.
	NodeHeartbeatEventReregistered = "Node reregistered by heartbeat"

	// NodeWaitingForNodePool is the message used when the node is waiting for
	// its node pool to be created.
	NodeWaitingForNodePool = "Node registered but waiting for node pool to be created"
)

// Node endpoint is used for client interactions
type Node struct {
	srv    *Server
	logger hclog.Logger

	// ctx provides context regarding the underlying connection
	ctx *RPCContext

	// updates holds pending client status updates for allocations
	updates []*structs.Allocation

	// evals holds pending rescheduling eval updates triggered by failed allocations
	evals []*structs.Evaluation

	// updateFuture is used to wait for the pending batch update
	// to complete. This may be nil if no batch is pending.
	updateFuture *structs.BatchFuture

	// updateTimer is the timer that will trigger the next batch
	// update, and may be nil if there is no batch pending.
	updateTimer *time.Timer

	// updatesLock synchronizes access to the updates list,
	// the future and the timer.
	updatesLock sync.Mutex
}

func NewNodeEndpoint(srv *Server, ctx *RPCContext) *Node {
	return &Node{
		srv:     srv,
		ctx:     ctx,
		logger:  srv.logger.Named("client"),
		updates: []*structs.Allocation{},
		evals:   []*structs.Evaluation{},
	}
}

// Register is used to upsert a client that is available for scheduling
func (n *Node) Register(args *structs.NodeRegisterRequest, reply *structs.NodeUpdateResponse) error {
	// note that we trust-on-first use and the identity will be anonymous for
	// that initial request; we lean on mTLS for handling that safely
	authErr := n.srv.Authenticate(n.ctx, args)

	isForwarded := args.IsForwarded()
	if done, err := n.srv.forward("Node.Register", args, args, reply); done {
		// We have a valid node connection since there is no error from the
		// forwarded server, so add the mapping to cache the
		// connection and allow the server to send RPCs to the client.
		if err == nil && n.ctx != nil && n.ctx.NodeID == "" && !isForwarded {
			n.ctx.NodeID = args.Node.ID
			n.srv.addNodeConn(n.ctx)
		}

		return err
	}
	n.srv.MeasureRPCRate("node", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}

	defer metrics.MeasureSince([]string{"nomad", "client", "register"}, time.Now())

	// Validate the arguments
	if args.Node == nil {
		return fmt.Errorf("missing node for client registration")
	}
	if args.Node.ID == "" {
		return fmt.Errorf("missing node ID for client registration")
	}
	if args.Node.Datacenter == "" {
		return fmt.Errorf("missing datacenter for client registration")
	}
	if args.Node.Name == "" {
		return fmt.Errorf("missing node name for client registration")
	}
	if len(args.Node.Attributes) == 0 {
		return fmt.Errorf("missing attributes for client registration")
	}
	if args.Node.SecretID == "" {
		return fmt.Errorf("missing node secret ID for client registration")
	}
	if args.Node.NodePool != "" {
		err := structs.ValidateNodePoolName(args.Node.NodePool)
		if err != nil {
			return fmt.Errorf("invalid node pool: %v", err)
		}
		if args.Node.NodePool == structs.NodePoolAll {
			return fmt.Errorf("node is not allowed to register in node pool %q", structs.NodePoolAll)
		}
	}

	// Default the status if none is given
	if args.Node.Status == "" {
		args.Node.Status = structs.NodeStatusInit
	}
	if !structs.ValidNodeStatus(args.Node.Status) {
		return fmt.Errorf("invalid status for node")
	}

	// Default to eligible for scheduling if unset
	if args.Node.SchedulingEligibility == "" {
		args.Node.SchedulingEligibility = structs.NodeSchedulingEligible
	}

	// Default the node pool if none is given.
	if args.Node.NodePool == "" {
		args.Node.NodePool = structs.NodePoolDefault
	}

	// Set the timestamp when the node is registered
	args.Node.StatusUpdatedAt = time.Now().Unix()

	// Compute the node class
	if err := args.Node.ComputeClass(); err != nil {
		return fmt.Errorf("failed to computed node class: %v", err)
	}

	// Look for the node so we can detect a state transition
	snap, err := n.srv.fsm.State().Snapshot()
	if err != nil {
		return err
	}

	ws := memdb.NewWatchSet()
	originalNode, err := snap.NodeByID(ws, args.Node.ID)
	if err != nil {
		return err
	}

	if originalNode != nil {
		// Check if the SecretID has been tampered with
		if args.Node.SecretID != originalNode.SecretID && originalNode.SecretID != "" {
			return fmt.Errorf("node secret ID does not match. Not registering node.")
		}

		// Don't allow the Register method to update the node status. Only the
		// UpdateStatus method should be able to do this.
		if originalNode.Status != "" {
			args.Node.Status = originalNode.Status
		}
	}

	// We have a valid node connection, so add the mapping to cache the
	// connection and allow the server to send RPCs to the client. We only cache
	// the connection if it is not being forwarded from another server.
	if n.ctx != nil && n.ctx.NodeID == "" && !args.IsForwarded() {
		n.ctx.NodeID = args.Node.ID
		n.srv.addNodeConn(n.ctx)
	}

	// Commit this update via Raft.
	//
	// Only the authoritative region is allowed to create the node pool for the
	// node if it doesn't exist yet. This prevents non-authoritative regions
	// from having to push their local state to the authoritative region.
	//
	// Nodes in non-authoritative regions that are registered with a new node
	// pool are kept in the `initializing` status until the node pool is
	// created and replicated.
	if n.srv.Region() == n.srv.config.AuthoritativeRegion {
		args.CreateNodePool = true
	}
	_, index, err := n.srv.raftApply(structs.NodeRegisterRequestType, args)
	if err != nil {
		n.logger.Error("register failed", "error", err)
		return err
	}
	reply.NodeModifyIndex = index

	// Check if we should trigger evaluations
	if shouldCreateNodeEval(originalNode, args.Node) {
		evalIDs, evalIndex, err := n.createNodeEvals(args.Node, index)
		if err != nil {
			n.logger.Error("eval creation failed", "error", err)
			return err
		}
		reply.EvalIDs = evalIDs
		reply.EvalCreateIndex = evalIndex
	}

	// Check if we need to setup a heartbeat
	if !args.Node.TerminalStatus() {
		ttl, err := n.srv.resetHeartbeatTimer(args.Node.ID)
		if err != nil {
			n.logger.Error("heartbeat reset failed", "error", err)
			return err
		}
		reply.HeartbeatTTL = ttl
	}

	// Set the reply index
	reply.Index = index
	snap, err = n.srv.fsm.State().Snapshot()
	if err != nil {
		return err
	}

	n.srv.peerLock.RLock()
	defer n.srv.peerLock.RUnlock()
	if err := n.constructNodeServerInfoResponse(args.Node.ID, snap, reply); err != nil {
		n.logger.Error("failed to populate NodeUpdateResponse", "error", err)
		return err
	}

	return nil
}

// shouldCreateNodeEval returns true if the node update may result into
// allocation updates, so the node should be re-evaluating.
//
// Such cases might be:
// * node health/drain status changes that may result into alloc rescheduling
// * node drivers or attributes changing that may cause system job placement changes
func shouldCreateNodeEval(original, updated *structs.Node) bool {
	if structs.ShouldDrainNode(updated.Status) {
		return true
	}

	if original == nil {
		return nodeStatusTransitionRequiresEval(updated.Status, structs.NodeStatusInit)
	}

	if nodeStatusTransitionRequiresEval(updated.Status, original.Status) {
		return true
	}

	// check fields used by the feasibility checks in ../scheduler/feasible.go,
	// whether through a Constraint explicitly added by user or an implicit constraint
	// added through a driver/volume check.
	//
	// Node Resources (e.g. CPU/Memory) are handled differently, using blocked evals,
	// and not relevant in this check.
	return !(original.ID == updated.ID &&
		original.Datacenter == updated.Datacenter &&
		original.Name == updated.Name &&
		original.NodeClass == updated.NodeClass &&
		reflect.DeepEqual(original.Attributes, updated.Attributes) &&
		reflect.DeepEqual(original.Meta, updated.Meta) &&
		reflect.DeepEqual(original.Drivers, updated.Drivers) &&
		reflect.DeepEqual(original.HostVolumes, updated.HostVolumes) &&
		equalDevices(original, updated))
}

func equalDevices(n1, n2 *structs.Node) bool {
	// ignore super old nodes, mostly to avoid nil dereferencing
	if n1.NodeResources == nil || n2.NodeResources == nil {
		return n1.NodeResources == n2.NodeResources
	}

	// treat nil and empty value as equal
	if len(n1.NodeResources.Devices) == 0 {
		return len(n1.NodeResources.Devices) == len(n2.NodeResources.Devices)
	}

	return reflect.DeepEqual(n1.NodeResources.Devices, n2.NodeResources.Devices)
}

// constructNodeServerInfoResponse assumes the n.srv.peerLock is held for reading.
func (n *Node) constructNodeServerInfoResponse(nodeID string, snap *state.StateSnapshot, reply *structs.NodeUpdateResponse) error {
	reply.LeaderRPCAddr = string(n.srv.raft.Leader())

	// Reply with config information required for future RPC requests
	reply.Servers = make([]*structs.NodeServerInfo, 0, len(n.srv.localPeers))
	for _, v := range n.srv.localPeers {
		reply.Servers = append(reply.Servers,
			&structs.NodeServerInfo{
				RPCAdvertiseAddr: v.RPCAddr.String(),
				Datacenter:       v.Datacenter,
			})
	}

	ws := memdb.NewWatchSet()

	// Add ClientStatus information to heartbeat response.
	if node, err := snap.NodeByID(ws, nodeID); err == nil && node != nil {
		reply.SchedulingEligibility = node.SchedulingEligibility
	} else if node == nil {

		// If the node is not found, leave reply.SchedulingEligibility as
		// the empty string. The response handler in the client treats this
		// as a no-op. As there is no call to action for an operator, log it
		// at debug level.
		n.logger.Debug("constructNodeServerInfoResponse: node not found",
			"node_id", nodeID)
	} else {

		// This case is likely only reached via a code error in state store
		return err
	}

	// TODO(sean@): Use an indexed node count instead
	//
	// Snapshot is used only to iterate over all nodes to create a node
	// count to send back to Nomad Clients in their heartbeat so Clients
	// can estimate the size of the cluster.
	iter, err := snap.Nodes(ws)
	if err == nil {
		for {
			raw := iter.Next()
			if raw == nil {
				break
			}
			reply.NumNodes++
		}
	}

	reply.Features = n.srv.EnterpriseState.Features()

	return nil
}

// Deregister is used to remove a client from the cluster. If a client should
// just be made unavailable for scheduling, a status update is preferred.
func (n *Node) Deregister(args *structs.NodeDeregisterRequest, reply *structs.NodeUpdateResponse) error {
	authErr := n.srv.Authenticate(n.ctx, args)
	if done, err := n.srv.forward("Node.Deregister", args, args, reply); done {
		return err
	}
	n.srv.MeasureRPCRate("node", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "client", "deregister"}, time.Now())

	if args.NodeID == "" {
		return fmt.Errorf("missing node ID for client deregistration")
	}

	// deregister takes a batch
	repack := &structs.NodeBatchDeregisterRequest{
		NodeIDs:      []string{args.NodeID},
		WriteRequest: args.WriteRequest,
	}

	return n.deregister(repack, reply, func() (interface{}, uint64, error) {
		return n.srv.raftApply(structs.NodeDeregisterRequestType, args)
	})
}

// BatchDeregister is used to remove client nodes from the cluster.
func (n *Node) BatchDeregister(args *structs.NodeBatchDeregisterRequest, reply *structs.NodeUpdateResponse) error {
	authErr := n.srv.Authenticate(n.ctx, args)
	if done, err := n.srv.forward("Node.BatchDeregister", args, args, reply); done {
		return err
	}
	n.srv.MeasureRPCRate("node", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "client", "batch_deregister"}, time.Now())

	if len(args.NodeIDs) == 0 {
		return fmt.Errorf("missing node IDs for client deregistration")
	}

	return n.deregister(args, reply, func() (interface{}, uint64, error) {
		return n.srv.raftApply(structs.NodeBatchDeregisterRequestType, args)
	})
}

// deregister takes a raftMessage closure, to support both Deregister and BatchDeregister
func (n *Node) deregister(args *structs.NodeBatchDeregisterRequest,
	reply *structs.NodeUpdateResponse,
	raftApplyFn func() (interface{}, uint64, error),
) error {
	// Check request permissions
	if aclObj, err := n.srv.ResolveACL(args); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowNodeWrite() {
		return structs.ErrPermissionDenied
	}

	// Look for the node
	snap, err := n.srv.fsm.State().Snapshot()
	if err != nil {
		return err
	}

	nodes := make([]*structs.Node, 0, len(args.NodeIDs))
	for _, nodeID := range args.NodeIDs {
		node, err := snap.NodeByID(nil, nodeID)
		if err != nil {
			return err
		}
		if node == nil {
			return fmt.Errorf("node not found")
		}
		nodes = append(nodes, node)
	}

	// Commit this update via Raft
	_, index, err := raftApplyFn()
	if err != nil {
		n.logger.Error("raft message failed", "error", err)
		return err
	}

	for _, node := range nodes {
		nodeID := node.ID

		// Clear the heartbeat timer if any
		n.srv.clearHeartbeatTimer(nodeID)

		// Create the evaluations for this node
		evalIDs, evalIndex, err := n.createNodeEvals(node, index)
		if err != nil {
			n.logger.Error("eval creation failed", "error", err)
			return err
		}

		// Determine if there are any Vault accessors on the node
		if accessors, err := snap.VaultAccessorsByNode(nil, nodeID); err != nil {
			n.logger.Error("looking up vault accessors for node failed", "node_id", nodeID, "error", err)
			return err
		} else if l := len(accessors); l > 0 {
			n.logger.Debug("revoking vault accessors on node due to deregister", "num_accessors", l, "node_id", nodeID)
			if err := n.srv.vault.RevokeTokens(context.Background(), accessors, true); err != nil {
				n.logger.Error("revoking vault accessors for node failed", "node_id", nodeID, "error", err)
				return err
			}
		}

		// Determine if there are any SI token accessors on the node
		if accessors, err := snap.SITokenAccessorsByNode(nil, nodeID); err != nil {
			n.logger.Error("looking up si accessors for node failed", "node_id", nodeID, "error", err)
			return err
		} else if l := len(accessors); l > 0 {
			n.logger.Debug("revoking si accessors on node due to deregister", "num_accessors", l, "node_id", nodeID)
			// Unlike with the Vault integration, there's no error returned here, since
			// bootstrapping the Consul client is elsewhere. Errors in revocation trigger
			// background retry attempts rather than inline error handling.
			_ = n.srv.consulACLs.RevokeTokens(context.Background(), accessors, true)
		}

		reply.EvalIDs = append(reply.EvalIDs, evalIDs...)
		// Set the reply eval create index just the first time
		if reply.EvalCreateIndex == 0 {
			reply.EvalCreateIndex = evalIndex
		}
	}

	reply.NodeModifyIndex = index
	reply.Index = index
	return nil
}

// UpdateStatus is used to update the status of a client node.
//
// Clients with non-terminal allocations must first call UpdateAlloc to be able
// to transition from the initializing status to ready.
//
// Clients node pool must exist for them to be able to transition from
// initializing to ready.
//
//	                ┌────────────────────────────────────── No ───┐
//	                │                                             │
//	             ┌──▼───┐          ┌─────────────┐       ┌────────┴────────┐
//	── Register ─► init ├─ ready ──► Has allocs? ├─ Yes ─► Allocs updated? │
//	             └──▲──▲┘          └─────┬───────┘       └────────┬────────┘
//	                │  │                 │                        │
//	                │  │                 └─ No ─┐  ┌─────── Yes ──┘
//	                │  │                        │  │
//	                │  │               ┌────────▼──▼───────┐
//	                │  └──────────No───┤ Node pool exists? │
//	                │                  └─────────┬─────────┘
//	                │                            │
//	              ready                         Yes
//	                │                            │
//	         ┌──────┴───────┐                ┌───▼───┐         ┌──────┐
//	         │ disconnected ◄─ disconnected ─┤ ready ├─ down ──► down │
//	         └──────────────┘                └───▲───┘         └──┬───┘
//	                                             │                │
//	                                             └──── ready ─────┘
func (n *Node) UpdateStatus(args *structs.NodeUpdateStatusRequest, reply *structs.NodeUpdateResponse) error {
	authErr := n.srv.Authenticate(n.ctx, args)

	isForwarded := args.IsForwarded()
	if done, err := n.srv.forward("Node.UpdateStatus", args, args, reply); done {
		// We have a valid node connection since there is no error from the
		// forwarded server, so add the mapping to cache the
		// connection and allow the server to send RPCs to the client.
		if err == nil && n.ctx != nil && n.ctx.NodeID == "" && !isForwarded {
			n.ctx.NodeID = args.NodeID
			n.srv.addNodeConn(n.ctx)
		}

		return err
	}
	n.srv.MeasureRPCRate("node", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}

	defer metrics.MeasureSince([]string{"nomad", "client", "update_status"}, time.Now())

	// Verify the arguments
	if args.NodeID == "" {
		return fmt.Errorf("missing node ID for client status update")
	}
	if !structs.ValidNodeStatus(args.Status) {
		return fmt.Errorf("invalid status for node")
	}

	// Look for the node
	snap, err := n.srv.fsm.State().Snapshot()
	if err != nil {
		return err
	}

	ws := memdb.NewWatchSet()
	node, err := snap.NodeByID(ws, args.NodeID)
	if err != nil {
		return err
	}
	if node == nil {
		return fmt.Errorf("node not found")
	}

	// We have a valid node connection, so add the mapping to cache the
	// connection and allow the server to send RPCs to the client. We only cache
	// the connection if it is not being forwarded from another server.
	if n.ctx != nil && n.ctx.NodeID == "" && !args.IsForwarded() {
		n.ctx.NodeID = args.NodeID
		n.srv.addNodeConn(n.ctx)
	}

	// XXX: Could use the SecretID here but have to update the heartbeat system
	// to track SecretIDs.

	// Update the timestamp of when the node status was updated
	args.UpdatedAt = time.Now().Unix()

	// Compute next status.
	switch node.Status {
	case structs.NodeStatusInit:
		if args.Status == structs.NodeStatusReady {
			// Keep node in the initializing status if it has allocations but
			// they are not updated.
			allocs, err := snap.AllocsByNodeTerminal(ws, args.NodeID, false)
			if err != nil {
				return fmt.Errorf("failed to query node allocs: %v", err)
			}

			allocsUpdated := node.LastAllocUpdateIndex > node.LastMissedHeartbeatIndex
			if len(allocs) > 0 && !allocsUpdated {
				n.logger.Debug(fmt.Sprintf("marking node as %s due to outdated allocation information", structs.NodeStatusInit))
				args.Status = structs.NodeStatusInit
			}

			// Keep node in the initialing status if it's in a node pool that
			// doesn't exist.
			pool, err := snap.NodePoolByName(ws, node.NodePool)
			if err != nil {
				return fmt.Errorf("failed to query node pool: %v", err)
			}
			if pool == nil {
				n.logger.Debug(fmt.Sprintf("marking node as %s due to missing node pool", structs.NodeStatusInit))
				args.Status = structs.NodeStatusInit
				if !node.HasEvent(NodeWaitingForNodePool) {
					args.NodeEvent = structs.NewNodeEvent().
						SetSubsystem(structs.NodeEventSubsystemCluster).
						SetMessage(NodeWaitingForNodePool).
						AddDetail("node_pool", node.NodePool)
				}
			}
		}
	case structs.NodeStatusDisconnected:
		if args.Status == structs.NodeStatusReady {
			args.Status = structs.NodeStatusInit
		}
	}

	// Commit this update via Raft
	var index uint64
	if node.Status != args.Status || args.NodeEvent != nil {
		// Attach an event if we are updating the node status to ready when it
		// is down via a heartbeat
		if node.Status == structs.NodeStatusDown && args.NodeEvent == nil {
			args.NodeEvent = structs.NewNodeEvent().
				SetSubsystem(structs.NodeEventSubsystemCluster).
				SetMessage(NodeHeartbeatEventReregistered)
		}

		_, index, err = n.srv.raftApply(structs.NodeUpdateStatusRequestType, args)
		if err != nil {
			n.logger.Error("status update failed", "error", err)
			return err
		}
		reply.NodeModifyIndex = index
	}

	// Check if we should trigger evaluations
	if structs.ShouldDrainNode(args.Status) ||
		nodeStatusTransitionRequiresEval(args.Status, node.Status) {
		evalIDs, evalIndex, err := n.createNodeEvals(node, index)
		if err != nil {
			n.logger.Error("eval creation failed", "error", err)
			return err
		}
		reply.EvalIDs = evalIDs
		reply.EvalCreateIndex = evalIndex
	}

	// Check if we need to setup a heartbeat
	switch args.Status {
	case structs.NodeStatusDown:
		// Determine if there are any Vault accessors on the node to cleanup
		if accessors, err := n.srv.State().VaultAccessorsByNode(ws, args.NodeID); err != nil {
			n.logger.Error("looking up vault accessors for node failed", "node_id", args.NodeID, "error", err)
			return err
		} else if l := len(accessors); l > 0 {
			n.logger.Debug("revoking vault accessors on node due to down state", "num_accessors", l, "node_id", args.NodeID)
			if err := n.srv.vault.RevokeTokens(context.Background(), accessors, true); err != nil {
				n.logger.Error("revoking vault accessors for node failed", "node_id", args.NodeID, "error", err)
				return err
			}
		}

		// Determine if there are any SI token accessors on the node to cleanup
		if accessors, err := n.srv.State().SITokenAccessorsByNode(ws, args.NodeID); err != nil {
			n.logger.Error("looking up SI accessors for node failed", "node_id", args.NodeID, "error", err)
			return err
		} else if l := len(accessors); l > 0 {
			n.logger.Debug("revoking SI accessors on node due to down state", "num_accessors", l, "node_id", args.NodeID)
			_ = n.srv.consulACLs.RevokeTokens(context.Background(), accessors, true)
		}

		// Identify the service registrations current placed on the downed
		// node.
		serviceRegistrations, err := n.srv.State().GetServiceRegistrationsByNodeID(ws, args.NodeID)
		if err != nil {
			n.logger.Error("looking up service registrations for node failed",
				"node_id", args.NodeID, "error", err)
			return err
		}

		// If the node has service registrations assigned to it, delete these
		// via Raft.
		if l := len(serviceRegistrations); l > 0 {
			n.logger.Debug("deleting service registrations on node due to down state",
				"num_service_registrations", l, "node_id", args.NodeID)

			deleteRegReq := structs.ServiceRegistrationDeleteByNodeIDRequest{NodeID: args.NodeID}

			_, index, err = n.srv.raftApply(structs.ServiceRegistrationDeleteByNodeIDRequestType, &deleteRegReq)
			if err != nil {
				n.logger.Error("failed to delete service registrations for node",
					"node_id", args.NodeID, "error", err)
				return err
			}
		}

	default:
		ttl, err := n.srv.resetHeartbeatTimer(args.NodeID)
		if err != nil {
			n.logger.Error("heartbeat reset failed", "error", err)
			return err
		}
		reply.HeartbeatTTL = ttl
	}

	// Set the reply index and leader
	reply.Index = index
	n.srv.peerLock.RLock()
	defer n.srv.peerLock.RUnlock()
	if err := n.constructNodeServerInfoResponse(node.GetID(), snap, reply); err != nil {
		n.logger.Error("failed to populate NodeUpdateResponse", "error", err)
		return err
	}

	return nil
}

// nodeStatusTransitionRequiresEval is a helper that takes a nodes new and old status and
// returns whether it has transitioned to ready.
func nodeStatusTransitionRequiresEval(newStatus, oldStatus string) bool {
	initToReady := oldStatus == structs.NodeStatusInit && newStatus == structs.NodeStatusReady
	terminalToReady := oldStatus == structs.NodeStatusDown && newStatus == structs.NodeStatusReady
	disconnectedToOther := oldStatus == structs.NodeStatusDisconnected && newStatus != structs.NodeStatusDisconnected
	otherToDisconnected := oldStatus != structs.NodeStatusDisconnected && newStatus == structs.NodeStatusDisconnected
	return initToReady || terminalToReady || disconnectedToOther || otherToDisconnected
}

// UpdateDrain is used to update the drain mode of a client node
func (n *Node) UpdateDrain(args *structs.NodeUpdateDrainRequest,
	reply *structs.NodeDrainUpdateResponse) error {

	authErr := n.srv.Authenticate(n.ctx, args)
	if done, err := n.srv.forward("Node.UpdateDrain", args, args, reply); done {
		return err
	}
	n.srv.MeasureRPCRate("node", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "client", "update_drain"}, time.Now())

	// Check node write permissions
	if aclObj, err := n.srv.ResolveACL(args); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowNodeWrite() {
		return structs.ErrPermissionDenied
	}

	// Verify the arguments
	if args.NodeID == "" {
		return fmt.Errorf("missing node ID for drain update")
	}
	if args.NodeEvent != nil {
		return fmt.Errorf("node event must not be set")
	}

	// Look for the node
	snap, err := n.srv.fsm.State().Snapshot()
	if err != nil {
		return err
	}
	node, err := snap.NodeByID(nil, args.NodeID)
	if err != nil {
		return err
	}
	if node == nil {
		return fmt.Errorf("node not found")
	}

	now := time.Now().UTC()

	// Update the timestamp of when the node status was updated
	args.UpdatedAt = now.Unix()

	// Setup drain strategy
	if args.DrainStrategy != nil {
		// Mark start time for the drain
		if node.DrainStrategy == nil {
			args.DrainStrategy.StartedAt = now
		} else {
			args.DrainStrategy.StartedAt = node.DrainStrategy.StartedAt
		}

		// Mark the deadline time
		if args.DrainStrategy.Deadline.Nanoseconds() > 0 {
			args.DrainStrategy.ForceDeadline = now.Add(args.DrainStrategy.Deadline)
		}
	}

	// Construct the node event
	args.NodeEvent = structs.NewNodeEvent().SetSubsystem(structs.NodeEventSubsystemDrain)
	if node.DrainStrategy == nil && args.DrainStrategy != nil {
		args.NodeEvent.SetMessage(NodeDrainEventDrainSet)
	} else if node.DrainStrategy != nil && args.DrainStrategy != nil {
		args.NodeEvent.SetMessage(NodeDrainEventDrainUpdated)
	} else if node.DrainStrategy != nil && args.DrainStrategy == nil {
		args.NodeEvent.SetMessage(NodeDrainEventDrainDisabled)
	} else {
		args.NodeEvent = nil
	}

	// Commit this update via Raft
	_, index, err := n.srv.raftApply(structs.NodeUpdateDrainRequestType, args)
	if err != nil {
		n.logger.Error("drain update failed", "error", err)
		return err
	}
	reply.NodeModifyIndex = index

	// If the node is transitioning to be eligible, create Node evaluations
	// because there may be a System job registered that should be evaluated.
	if node.SchedulingEligibility == structs.NodeSchedulingIneligible && args.MarkEligible && args.DrainStrategy == nil {
		n.logger.Info("node transitioning to eligible state", "node_id", node.ID)
		evalIDs, evalIndex, err := n.createNodeEvals(node, index)
		if err != nil {
			n.logger.Error("eval creation failed", "error", err)
			return err
		}
		reply.EvalIDs = evalIDs
		reply.EvalCreateIndex = evalIndex
	}

	// Set the reply index
	reply.Index = index
	return nil
}

// UpdateEligibility is used to update the scheduling eligibility of a node
func (n *Node) UpdateEligibility(args *structs.NodeUpdateEligibilityRequest,
	reply *structs.NodeEligibilityUpdateResponse) error {

	authErr := n.srv.Authenticate(n.ctx, args)
	if done, err := n.srv.forward("Node.UpdateEligibility", args, args, reply); done {
		return err
	}
	n.srv.MeasureRPCRate("node", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "client", "update_eligibility"}, time.Now())

	// Check node write permissions
	if aclObj, err := n.srv.ResolveACL(args); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowNodeWrite() {
		return structs.ErrPermissionDenied
	}

	// Verify the arguments
	if args.NodeID == "" {
		return fmt.Errorf("missing node ID for setting scheduling eligibility")
	}
	if args.NodeEvent != nil {
		return fmt.Errorf("node event must not be set")
	}

	// Check that only allowed types are set
	switch args.Eligibility {
	case structs.NodeSchedulingEligible, structs.NodeSchedulingIneligible:
	default:
		return fmt.Errorf("invalid scheduling eligibility %q", args.Eligibility)
	}

	// Look for the node
	snap, err := n.srv.fsm.State().Snapshot()
	if err != nil {
		return err
	}
	node, err := snap.NodeByID(nil, args.NodeID)
	if err != nil {
		return err
	}
	if node == nil {
		return fmt.Errorf("node not found")
	}

	if node.DrainStrategy != nil && args.Eligibility == structs.NodeSchedulingEligible {
		return fmt.Errorf("can not set node's scheduling eligibility to eligible while it is draining")
	}

	switch args.Eligibility {
	case structs.NodeSchedulingEligible, structs.NodeSchedulingIneligible:
	default:
		return fmt.Errorf("invalid scheduling eligibility %q", args.Eligibility)
	}

	// Update the timestamp of when the node status was updated
	args.UpdatedAt = time.Now().Unix()

	// Construct the node event
	args.NodeEvent = structs.NewNodeEvent().SetSubsystem(structs.NodeEventSubsystemCluster)
	if node.SchedulingEligibility == args.Eligibility {
		return nil // Nothing to do
	} else if args.Eligibility == structs.NodeSchedulingEligible {
		n.logger.Info("node transitioning to eligible state", "node_id", node.ID)
		args.NodeEvent.SetMessage(NodeEligibilityEventEligible)
	} else {
		n.logger.Info("node transitioning to ineligible state", "node_id", node.ID)
		args.NodeEvent.SetMessage(NodeEligibilityEventIneligible)
	}

	// Commit this update via Raft
	outErr, index, err := n.srv.raftApply(structs.NodeUpdateEligibilityRequestType, args)
	if err != nil {
		n.logger.Error("eligibility update failed", "error", err)
		return err
	}
	if outErr != nil {
		if err, ok := outErr.(error); ok && err != nil {
			n.logger.Error("eligibility update failed", "error", err)
			return err
		}
	}

	// If the node is transitioning to be eligible, create Node evaluations
	// because there may be a System job registered that should be evaluated.
	if node.SchedulingEligibility == structs.NodeSchedulingIneligible && args.Eligibility == structs.NodeSchedulingEligible {
		evalIDs, evalIndex, err := n.createNodeEvals(node, index)
		if err != nil {
			n.logger.Error("eval creation failed", "error", err)
			return err
		}
		reply.EvalIDs = evalIDs
		reply.EvalCreateIndex = evalIndex
	}

	// Set the reply index
	reply.Index = index
	return nil
}

// Evaluate is used to force a re-evaluation of the node
func (n *Node) Evaluate(args *structs.NodeEvaluateRequest, reply *structs.NodeUpdateResponse) error {

	authErr := n.srv.Authenticate(n.ctx, args)
	if done, err := n.srv.forward("Node.Evaluate", args, args, reply); done {
		return err
	}
	n.srv.MeasureRPCRate("node", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "client", "evaluate"}, time.Now())

	// Check node write permissions
	if aclObj, err := n.srv.ResolveACL(args); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowNodeWrite() {
		return structs.ErrPermissionDenied
	}

	// Verify the arguments
	if args.NodeID == "" {
		return fmt.Errorf("missing node ID for evaluation")
	}

	// Look for the node
	snap, err := n.srv.fsm.State().Snapshot()
	if err != nil {
		return err
	}
	ws := memdb.NewWatchSet()
	node, err := snap.NodeByID(ws, args.NodeID)
	if err != nil {
		return err
	}
	if node == nil {
		return fmt.Errorf("node not found")
	}

	// Create the evaluation
	evalIDs, evalIndex, err := n.createNodeEvals(node, node.ModifyIndex)
	if err != nil {
		n.logger.Error("eval creation failed", "error", err)
		return err
	}
	reply.EvalIDs = evalIDs
	reply.EvalCreateIndex = evalIndex

	// Set the reply index
	reply.Index = evalIndex

	n.srv.peerLock.RLock()
	defer n.srv.peerLock.RUnlock()
	if err := n.constructNodeServerInfoResponse(node.GetID(), snap, reply); err != nil {
		n.logger.Error("failed to populate NodeUpdateResponse", "error", err)
		return err
	}
	return nil
}

// GetNode is used to request information about a specific node
func (n *Node) GetNode(args *structs.NodeSpecificRequest,
	reply *structs.SingleNodeResponse) error {

	authErr := n.srv.Authenticate(n.ctx, args)
	if done, err := n.srv.forward("Node.GetNode", args, args, reply); done {
		return err
	}
	n.srv.MeasureRPCRate("node", structs.RateMetricRead, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "client", "get_node"}, time.Now())

	// Check node read permissions
	aclObj, err := n.srv.ResolveClientOrACL(args)
	if err != nil {
		return err
	}
	if aclObj != nil && !aclObj.AllowNodeRead() {
		return structs.ErrPermissionDenied
	}

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			// Verify the arguments
			if args.NodeID == "" {
				return fmt.Errorf("missing node ID")
			}

			// Look for the node
			out, err := state.NodeByID(ws, args.NodeID)
			if err != nil {
				return err
			}

			// Setup the output
			if out != nil {
				out = out.Sanitize()
				reply.Node = out
				reply.Index = out.ModifyIndex
			} else {
				// Use the last index that affected the nodes table
				index, err := state.Index("nodes")
				if err != nil {
					return err
				}
				reply.Node = nil
				reply.Index = index
			}

			// Set the query response
			n.srv.setQueryMeta(&reply.QueryMeta)
			return nil
		}}
	return n.srv.blockingRPC(&opts)
}

// GetAllocs is used to request allocations for a specific node
func (n *Node) GetAllocs(args *structs.NodeSpecificRequest,
	reply *structs.NodeAllocsResponse) error {

	authErr := n.srv.Authenticate(n.ctx, args)
	if done, err := n.srv.forward("Node.GetAllocs", args, args, reply); done {
		return err
	}
	n.srv.MeasureRPCRate("node", structs.RateMetricList, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "client", "get_allocs"}, time.Now())

	// Check node read and namespace job read permissions
	aclObj, err := n.srv.ResolveACL(args)
	if err != nil {
		return err
	}
	if aclObj != nil && !aclObj.AllowNodeRead() {
		return structs.ErrPermissionDenied
	}

	// cache namespace perms
	readableNamespaces := map[string]bool{}

	// readNS is a caching namespace read-job helper
	readNS := func(ns string) bool {
		if aclObj == nil {
			// ACLs are disabled; everything is readable
			return true
		}

		if readable, ok := readableNamespaces[ns]; ok {
			// cache hit
			return readable
		}

		// cache miss
		readable := aclObj.AllowNsOp(ns, acl.NamespaceCapabilityReadJob)
		readableNamespaces[ns] = readable
		return readable
	}

	// Verify the arguments
	if args.NodeID == "" {
		return fmt.Errorf("missing node ID")
	}

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			// Look for the node
			allocs, err := state.AllocsByNode(ws, args.NodeID)
			if err != nil {
				return err
			}

			// Setup the output
			if n := len(allocs); n != 0 {
				reply.Allocs = make([]*structs.Allocation, 0, n)
				for _, alloc := range allocs {
					if readNS(alloc.Namespace) {
						reply.Allocs = append(reply.Allocs, alloc)
					}

					// Get the max of all allocs since
					// subsequent requests need to start
					// from the latest index
					reply.Index = maxUint64(reply.Index, alloc.ModifyIndex)
				}
			} else {
				reply.Allocs = nil

				// Use the last index that affected the nodes table
				index, err := state.Index("allocs")
				if err != nil {
					return err
				}

				// Must provide non-zero index to prevent blocking
				// Index 1 is impossible anyways (due to Raft internals)
				if index == 0 {
					reply.Index = 1
				} else {
					reply.Index = index
				}
			}
			return nil
		}}
	return n.srv.blockingRPC(&opts)
}

// GetClientAllocs is used to request a lightweight list of alloc modify indexes
// per allocation.
func (n *Node) GetClientAllocs(args *structs.NodeSpecificRequest,
	reply *structs.NodeClientAllocsResponse) error {

	authErr := n.srv.Authenticate(n.ctx, args)
	isForwarded := args.IsForwarded()
	if done, err := n.srv.forward("Node.GetClientAllocs", args, args, reply); done {
		// We have a valid node connection since there is no error from the
		// forwarded server, so add the mapping to cache the
		// connection and allow the server to send RPCs to the client.
		if err == nil && n.ctx != nil && n.ctx.NodeID == "" && !isForwarded {
			n.ctx.NodeID = args.NodeID
			n.srv.addNodeConn(n.ctx)
		}

		return err
	}
	n.srv.MeasureRPCRate("node", structs.RateMetricList, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "client", "get_client_allocs"}, time.Now())

	// Verify the arguments
	if args.NodeID == "" {
		return fmt.Errorf("missing node ID")
	}

	// numOldAllocs is used to detect if there is a garbage collection event
	// that effects the node. When an allocation is garbage collected, that does
	// not change the modify index changes and thus the query won't unblock,
	// even though the set of allocations on the node has changed.
	var numOldAllocs int

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			// Look for the node
			node, err := state.NodeByID(ws, args.NodeID)
			if err != nil {
				return err
			}

			var allocs []*structs.Allocation
			if node != nil {
				if args.SecretID == "" {
					return fmt.Errorf("missing node secret ID for client status update")
				} else if args.SecretID != node.SecretID {
					return fmt.Errorf("node secret ID does not match")
				}

				// We have a valid node connection, so add the mapping to cache the
				// connection and allow the server to send RPCs to the client. We only cache
				// the connection if it is not being forwarded from another server.
				if n.ctx != nil && n.ctx.NodeID == "" && !args.IsForwarded() {
					n.ctx.NodeID = args.NodeID
					n.srv.addNodeConn(n.ctx)
				}

				var err error
				allocs, err = state.AllocsByNode(ws, args.NodeID)
				if err != nil {
					return err
				}
			}

			reply.Allocs = make(map[string]uint64)
			reply.MigrateTokens = make(map[string]string)

			// preferTableIndex is used to determine whether we should build the
			// response index based on the full table indexes versus the modify
			// indexes of the allocations on the specific node. This is
			// preferred in the case that the node doesn't yet have allocations
			// or when we detect a GC that effects the node.
			preferTableIndex := true

			// Setup the output
			if numAllocs := len(allocs); numAllocs != 0 {
				preferTableIndex = false

				for _, alloc := range allocs {
					reply.Allocs[alloc.ID] = alloc.AllocModifyIndex

					// If the allocation is going to do a migration, create a
					// migration token so that the client can authenticate with
					// the node hosting the previous allocation.
					if alloc.ShouldMigrate() {
						prevAllocation, err := state.AllocByID(ws, alloc.PreviousAllocation)
						if err != nil {
							return err
						}

						if prevAllocation != nil && prevAllocation.NodeID != alloc.NodeID {
							allocNode, err := state.NodeByID(ws, prevAllocation.NodeID)
							if err != nil {
								return err
							}
							if allocNode == nil {
								// Node must have been GC'd so skip the token
								continue
							}

							token, err := structs.GenerateMigrateToken(prevAllocation.ID, allocNode.SecretID)
							if err != nil {
								return err
							}
							reply.MigrateTokens[alloc.ID] = token
						}
					}

					reply.Index = maxUint64(reply.Index, alloc.ModifyIndex)
				}

				// Determine if we have less allocations than before. This
				// indicates there was a garbage collection
				if numAllocs < numOldAllocs {
					preferTableIndex = true
				}

				// Store the new number of allocations
				numOldAllocs = numAllocs
			}

			if preferTableIndex {
				// Use the last index that affected the nodes table
				index, err := state.Index("allocs")
				if err != nil {
					return err
				}

				// Must provide non-zero index to prevent blocking
				// Index 1 is impossible anyways (due to Raft internals)
				if index == 0 {
					reply.Index = 1
				} else {
					reply.Index = index
				}
			}
			return nil
		}}
	return n.srv.blockingRPC(&opts)
}

// UpdateAlloc is used to update the client status of an allocation. It should
// only be called by clients.
//
// Calling this method returns an error when:
//   - The node is not registered in the server yet. Clients must first call the
//     Register method.
//   - The node status is down or disconnected. Clients must call the
//     UpdateStatus method to update its status in the server.
func (n *Node) UpdateAlloc(args *structs.AllocUpdateRequest, reply *structs.GenericResponse) error {

	authErr := n.srv.Authenticate(n.ctx, args)

	// Ensure the connection was initiated by another client if TLS is used.
	err := validateTLSCertificateLevel(n.srv, n.ctx, tlsCertificateLevelClient)
	if err != nil {
		return err
	}
	if done, err := n.srv.forward("Node.UpdateAlloc", args, args, reply); done {
		return err
	}
	n.srv.MeasureRPCRate("node", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}

	defer metrics.MeasureSince([]string{"nomad", "client", "update_alloc"}, time.Now())

	// Ensure at least a single alloc
	if len(args.Alloc) == 0 {
		return fmt.Errorf("must update at least one allocation")
	}

	// Ensure the node is allowed to update allocs.
	// The node needs to successfully heartbeat before updating its allocs.
	nodeID := args.Alloc[0].NodeID
	if nodeID == "" {
		return fmt.Errorf("missing node ID")
	}

	node, err := n.srv.State().NodeByID(nil, nodeID)
	if err != nil {
		return fmt.Errorf("failed to retrieve node %s: %v", nodeID, err)
	}
	if node == nil {
		return fmt.Errorf("node %s not found", nodeID)
	}
	if node.UnresponsiveStatus() {
		return fmt.Errorf("node %s is not allowed to update allocs while in status %s", nodeID, node.Status)
	}

	// Ensure that evals aren't set from client RPCs
	// We create them here before the raft update
	if len(args.Evals) != 0 {
		return fmt.Errorf("evals field must not be set")
	}

	// Update modified timestamp for client initiated allocation updates
	now := time.Now()
	var evals []*structs.Evaluation

	for _, allocToUpdate := range args.Alloc {
		evalTriggerBy := ""
		allocToUpdate.ModifyTime = now.UTC().UnixNano()

		alloc, _ := n.srv.State().AllocByID(nil, allocToUpdate.ID)
		if alloc == nil {
			continue
		}

		if !allocToUpdate.TerminalStatus() && alloc.ClientStatus != structs.AllocClientStatusUnknown {
			continue
		}

		var job *structs.Job
		var jobType string
		var jobPriority int

		job, err = n.srv.State().JobByID(nil, alloc.Namespace, alloc.JobID)
		if err != nil {
			n.logger.Debug("UpdateAlloc unable to find job", "job", alloc.JobID, "error", err)
			continue
		}

		// If the job is nil it means it has been de-registered.
		if job == nil {
			jobType = alloc.Job.Type
			jobPriority = alloc.Job.Priority
			evalTriggerBy = structs.EvalTriggerJobDeregister
			allocToUpdate.DesiredStatus = structs.AllocDesiredStatusStop
			n.logger.Debug("UpdateAlloc unable to find job - shutting down alloc", "job", alloc.JobID)
		}

		var taskGroup *structs.TaskGroup
		if job != nil {
			jobType = job.Type
			jobPriority = job.Priority
			taskGroup = job.LookupTaskGroup(alloc.TaskGroup)
		}

		// If we cannot find the task group for a failed alloc we cannot continue, unless it is an orphan.
		if evalTriggerBy != structs.EvalTriggerJobDeregister &&
			allocToUpdate.ClientStatus == structs.AllocClientStatusFailed &&
			alloc.FollowupEvalID == "" {

			if taskGroup == nil {
				n.logger.Debug("UpdateAlloc unable to find task group for job", "job", alloc.JobID, "alloc", alloc.ID, "task_group", alloc.TaskGroup)
				continue
			}

			// Set trigger by failed if not an orphan.
			if alloc.RescheduleEligible(taskGroup.ReschedulePolicy, now) {
				evalTriggerBy = structs.EvalTriggerRetryFailedAlloc
			}
		}

		var eval *structs.Evaluation
		// If unknown, and not an orphan, set the trigger by.
		if evalTriggerBy != structs.EvalTriggerJobDeregister &&
			alloc.ClientStatus == structs.AllocClientStatusUnknown {
			evalTriggerBy = structs.EvalTriggerReconnect
		}

		// If we weren't able to determine one of our expected eval triggers,
		// continue and don't create an eval.
		if evalTriggerBy == "" {
			continue
		}

		eval = &structs.Evaluation{
			ID:          uuid.Generate(),
			Namespace:   alloc.Namespace,
			TriggeredBy: evalTriggerBy,
			JobID:       alloc.JobID,
			Type:        jobType,
			Priority:    jobPriority,
			Status:      structs.EvalStatusPending,
			CreateTime:  now.UTC().UnixNano(),
			ModifyTime:  now.UTC().UnixNano(),
		}
		evals = append(evals, eval)
	}

	// Add this to the batch
	n.updatesLock.Lock()
	n.updates = append(n.updates, args.Alloc...)
	n.evals = append(n.evals, evals...)

	// Start a new batch if none
	future := n.updateFuture
	if future == nil {
		future = structs.NewBatchFuture()
		n.updateFuture = future
		n.updateTimer = time.AfterFunc(batchUpdateInterval, func() {
			// Get the pending updates
			n.updatesLock.Lock()
			updates := n.updates
			evals := n.evals
			future := n.updateFuture

			// Assume future update patterns will be similar to
			// current batch and set cap appropriately to avoid
			// slice resizing.
			n.updates = make([]*structs.Allocation, 0, len(updates))
			n.evals = make([]*structs.Evaluation, 0, len(evals))

			n.updateFuture = nil
			n.updateTimer = nil
			n.updatesLock.Unlock()

			// Perform the batch update
			n.batchUpdate(future, updates, evals)
		})
	}
	n.updatesLock.Unlock()

	// Wait for the future
	if err := future.Wait(); err != nil {
		return err
	}

	// Setup the response
	reply.Index = future.Index()
	return nil
}

// batchUpdate is used to update all the allocations
func (n *Node) batchUpdate(future *structs.BatchFuture, updates []*structs.Allocation, evals []*structs.Evaluation) {
	var mErr multierror.Error
	// Group pending evals by jobID to prevent creating unnecessary evals
	evalsByJobId := make(map[structs.NamespacedID]struct{})
	var trimmedEvals []*structs.Evaluation
	for _, eval := range evals {
		namespacedID := structs.NamespacedID{
			ID:        eval.JobID,
			Namespace: eval.Namespace,
		}
		_, exists := evalsByJobId[namespacedID]
		if !exists {
			now := time.Now().UTC().UnixNano()
			eval.CreateTime = now
			eval.ModifyTime = now
			trimmedEvals = append(trimmedEvals, eval)
			evalsByJobId[namespacedID] = struct{}{}
		}
	}

	if len(trimmedEvals) > 0 {
		n.logger.Debug("adding evaluations for rescheduling failed allocations", "num_evals", len(trimmedEvals))
	}
	// Prepare the batch update
	batch := &structs.AllocUpdateRequest{
		Alloc:        updates,
		Evals:        trimmedEvals,
		WriteRequest: structs.WriteRequest{Region: n.srv.config.Region},
	}

	// Commit this update via Raft
	_, index, err := n.srv.raftApply(structs.AllocClientUpdateRequestType, batch)
	if err != nil {
		n.logger.Error("alloc update failed", "error", err)
		mErr.Errors = append(mErr.Errors, err)
	}

	// For each allocation we are updating, check if we should revoke any
	// - Vault token accessors
	// - Service Identity token accessors
	var (
		revokeVault []*structs.VaultAccessor
		revokeSI    []*structs.SITokenAccessor
	)

	for _, alloc := range updates {
		// Skip any allocation that isn't dead on the client
		if !alloc.Terminated() {
			continue
		}

		ws := memdb.NewWatchSet()

		// Determine if there are any orphaned Vault accessors for the allocation
		if accessors, err := n.srv.State().VaultAccessorsByAlloc(ws, alloc.ID); err != nil {
			n.logger.Error("looking up vault accessors for alloc failed", "alloc_id", alloc.ID, "error", err)
			mErr.Errors = append(mErr.Errors, err)
		} else {
			revokeVault = append(revokeVault, accessors...)
		}

		// Determine if there are any orphaned SI accessors for the allocation
		if accessors, err := n.srv.State().SITokenAccessorsByAlloc(ws, alloc.ID); err != nil {
			n.logger.Error("looking up si accessors for alloc failed", "alloc_id", alloc.ID, "error", err)
			mErr.Errors = append(mErr.Errors, err)
		} else {
			revokeSI = append(revokeSI, accessors...)
		}
	}

	// Revoke any orphaned Vault token accessors
	if l := len(revokeVault); l > 0 {
		n.logger.Debug("revoking vault accessors due to terminal allocations", "num_accessors", l)
		if err := n.srv.vault.RevokeTokens(context.Background(), revokeVault, true); err != nil {
			n.logger.Error("batched vault accessor revocation failed", "error", err)
			mErr.Errors = append(mErr.Errors, err)
		}
	}

	// Revoke any orphaned SI token accessors
	if l := len(revokeSI); l > 0 {
		n.logger.Debug("revoking si accessors due to terminal allocations", "num_accessors", l)
		_ = n.srv.consulACLs.RevokeTokens(context.Background(), revokeSI, true)
	}

	// Respond to the future
	future.Respond(index, mErr.ErrorOrNil())
}

// List is used to list the available nodes
func (n *Node) List(args *structs.NodeListRequest,
	reply *structs.NodeListResponse) error {

	authErr := n.srv.Authenticate(n.ctx, args)
	if done, err := n.srv.forward("Node.List", args, args, reply); done {
		return err
	}
	n.srv.MeasureRPCRate("node", structs.RateMetricList, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "client", "list"}, time.Now())

	// Check node read permissions
	if aclObj, err := n.srv.ResolveACL(args); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowNodeRead() {
		return structs.ErrPermissionDenied
	}

	// Set up the blocking query.
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {

			var err error
			var iter memdb.ResultIterator
			if prefix := args.QueryOptions.Prefix; prefix != "" {
				iter, err = state.NodesByIDPrefix(ws, prefix)
			} else {
				iter, err = state.Nodes(ws)
			}
			if err != nil {
				return err
			}

			// Generate the tokenizer to use for pagination using the populated
			// paginatorOpts object. The ID of a node must be unique within the
			// region, therefore we only need WithID on the paginator options.
			tokenizer := paginator.NewStructsTokenizer(iter, paginator.StructsTokenizerOptions{WithID: true})

			var nodes []*structs.NodeListStub

			// Build the paginator. This includes the function that is
			// responsible for appending a node to the nodes array.
			paginatorImpl, err := paginator.NewPaginator(iter, tokenizer, nil, args.QueryOptions,
				func(raw interface{}) error {
					nodes = append(nodes, raw.(*structs.Node).Stub(args.Fields))
					return nil
				})
			if err != nil {
				return structs.NewErrRPCCodedf(
					http.StatusBadRequest, "failed to create result paginator: %v", err)
			}

			// Calling page populates our output nodes array as well as returns
			// the next token.
			nextToken, err := paginatorImpl.Page()
			if err != nil {
				return structs.NewErrRPCCodedf(
					http.StatusBadRequest, "failed to read result page: %v", err)
			}

			// Populate the reply.
			reply.Nodes = nodes
			reply.NextToken = nextToken

			// Use the last index that affected the jobs table
			index, err := state.Index("nodes")
			if err != nil {
				return err
			}
			reply.Index = index

			// Set the query response
			n.srv.setQueryMeta(&reply.QueryMeta)
			return nil
		}}
	return n.srv.blockingRPC(&opts)
}

// createNodeEvals is used to create evaluations for each alloc on a node.
// Each Eval is scoped to a job, so we need to potentially trigger many evals.
func (n *Node) createNodeEvals(node *structs.Node, nodeIndex uint64) ([]string, uint64, error) {
	nodeID := node.ID

	// Snapshot the state
	snap, err := n.srv.fsm.State().Snapshot()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to snapshot state: %v", err)
	}

	// Find all the allocations for this node
	allocs, err := snap.AllocsByNode(nil, nodeID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to find allocs for '%s': %v", nodeID, err)
	}

	sysJobsIter, err := snap.JobsByScheduler(nil, "system")
	if err != nil {
		return nil, 0, fmt.Errorf("failed to find system jobs for '%s': %v", nodeID, err)
	}

	var sysJobs []*structs.Job
	for jobI := sysJobsIter.Next(); jobI != nil; jobI = sysJobsIter.Next() {
		job := jobI.(*structs.Job)
		// Avoid creating evals for jobs that don't run in this datacenter or
		// node pool. We could perform an entire feasibility check here, but
		// datacenter/pool is a good optimization to start with as their
		// cardinality tends to be low so the check shouldn't add much work.
		if node.IsInPool(job.NodePool) && node.IsInAnyDC(job.Datacenters) {
			sysJobs = append(sysJobs, job)
		}
	}

	// Fast-path if nothing to do
	if len(allocs) == 0 && len(sysJobs) == 0 {
		return nil, 0, nil
	}

	// Create an eval for each JobID affected
	var evals []*structs.Evaluation
	var evalIDs []string
	jobIDs := map[structs.NamespacedID]struct{}{}
	now := time.Now().UTC().UnixNano()

	for _, alloc := range allocs {
		// Deduplicate on JobID
		if _, ok := jobIDs[alloc.JobNamespacedID()]; ok {
			continue
		}
		jobIDs[alloc.JobNamespacedID()] = struct{}{}

		// Create a new eval
		eval := &structs.Evaluation{
			ID:              uuid.Generate(),
			Namespace:       alloc.Namespace,
			Priority:        alloc.Job.Priority,
			Type:            alloc.Job.Type,
			TriggeredBy:     structs.EvalTriggerNodeUpdate,
			JobID:           alloc.JobID,
			NodeID:          nodeID,
			NodeModifyIndex: nodeIndex,
			Status:          structs.EvalStatusPending,
			CreateTime:      now,
			ModifyTime:      now,
		}

		evals = append(evals, eval)
		evalIDs = append(evalIDs, eval.ID)
	}

	// Create an evaluation for each system job.
	for _, job := range sysJobs {
		// Still dedup on JobID as the node may already have the system job.
		if _, ok := jobIDs[job.NamespacedID()]; ok {
			continue
		}
		jobIDs[job.NamespacedID()] = struct{}{}

		// Create a new eval
		eval := &structs.Evaluation{
			ID:              uuid.Generate(),
			Namespace:       job.Namespace,
			Priority:        job.Priority,
			Type:            job.Type,
			TriggeredBy:     structs.EvalTriggerNodeUpdate,
			JobID:           job.ID,
			NodeID:          nodeID,
			NodeModifyIndex: nodeIndex,
			Status:          structs.EvalStatusPending,
			CreateTime:      now,
			ModifyTime:      now,
		}
		evals = append(evals, eval)
		evalIDs = append(evalIDs, eval.ID)
	}

	// Create the Raft transaction
	update := &structs.EvalUpdateRequest{
		Evals:        evals,
		WriteRequest: structs.WriteRequest{Region: n.srv.config.Region},
	}

	// Commit this evaluation via Raft
	// XXX: There is a risk of partial failure where the node update succeeds
	// but that the EvalUpdate does not.
	_, evalIndex, err := n.srv.raftApply(structs.EvalUpdateRequestType, update)
	if err != nil {
		return nil, 0, err
	}
	return evalIDs, evalIndex, nil
}

// DeriveVaultToken is used by the clients to request wrapped Vault tokens for
// tasks
func (n *Node) DeriveVaultToken(args *structs.DeriveVaultTokenRequest, reply *structs.DeriveVaultTokenResponse) error {

	authErr := n.srv.Authenticate(n.ctx, args)

	setError := func(e error, recoverable bool) {
		if e != nil {
			if re, ok := e.(*structs.RecoverableError); ok {
				reply.Error = re // No need to wrap if error is already a RecoverableError
			} else {
				reply.Error = structs.NewRecoverableError(e, recoverable).(*structs.RecoverableError)
			}
			n.logger.Error("DeriveVaultToken failed", "recoverable", recoverable, "error", e)
		}
	}

	if done, err := n.srv.forward("Node.DeriveVaultToken", args, args, reply); done {
		setError(err, structs.IsRecoverable(err) || err == structs.ErrNoLeader)
		return nil
	}
	n.srv.MeasureRPCRate("node", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "client", "derive_vault_token"}, time.Now())

	// Verify the arguments
	if args.NodeID == "" {
		setError(fmt.Errorf("missing node ID"), false)
		return nil
	}
	if args.SecretID == "" {
		setError(fmt.Errorf("missing node SecretID"), false)
		return nil
	}
	if args.AllocID == "" {
		setError(fmt.Errorf("missing allocation ID"), false)
		return nil
	}
	if len(args.Tasks) == 0 {
		setError(fmt.Errorf("no tasks specified"), false)
		return nil
	}

	// Verify the following:
	// * The Node exists and has the correct SecretID
	// * The Allocation exists on the specified Node
	// * The Allocation contains the given tasks and they each require Vault
	//   tokens
	snap, err := n.srv.fsm.State().Snapshot()
	if err != nil {
		setError(err, false)
		return nil
	}
	ws := memdb.NewWatchSet()
	node, err := snap.NodeByID(ws, args.NodeID)
	if err != nil {
		setError(err, false)
		return nil
	}
	if node == nil {
		setError(fmt.Errorf("Node %q does not exist", args.NodeID), false)
		return nil
	}
	if node.SecretID != args.SecretID {
		setError(fmt.Errorf("SecretID mismatch"), false)
		return nil
	}

	alloc, err := snap.AllocByID(ws, args.AllocID)
	if err != nil {
		setError(err, false)
		return nil
	}
	if alloc == nil {
		setError(fmt.Errorf("Allocation %q does not exist", args.AllocID), false)
		return nil
	}
	if alloc.NodeID != args.NodeID {
		setError(fmt.Errorf("Allocation %q not running on Node %q", args.AllocID, args.NodeID), false)
		return nil
	}
	if alloc.TerminalStatus() {
		setError(fmt.Errorf("Can't request Vault token for terminal allocation"), false)
		return nil
	}

	// Check if alloc has Vault
	vaultBlocks := alloc.Job.Vault()
	if vaultBlocks == nil {
		setError(fmt.Errorf("Job does not require Vault token"), false)
		return nil
	}
	tg, ok := vaultBlocks[alloc.TaskGroup]
	if !ok {
		setError(fmt.Errorf("Task group does not require Vault token"), false)
		return nil
	}

	var unneeded []string
	for _, task := range args.Tasks {
		taskVault := tg[task]
		if taskVault == nil || len(taskVault.Policies) == 0 {
			unneeded = append(unneeded, task)
		}
	}

	if len(unneeded) != 0 {
		e := fmt.Errorf("Requested Vault tokens for tasks without defined Vault policies: %s",
			strings.Join(unneeded, ", "))
		setError(e, false)
		return nil
	}

	// At this point the request is valid and we should contact Vault for
	// tokens.

	// Create an error group where we will spin up a fixed set of goroutines to
	// handle deriving tokens but where if any fails the whole group is
	// canceled.
	g, ctx := errgroup.WithContext(context.Background())

	// Cap the handlers
	handlers := len(args.Tasks)
	if handlers > maxParallelRequestsPerDerive {
		handlers = maxParallelRequestsPerDerive
	}

	// Create the Vault Tokens
	input := make(chan string, handlers)
	results := make(map[string]*vapi.Secret, len(args.Tasks))
	for i := 0; i < handlers; i++ {
		g.Go(func() error {
			for {
				select {
				case task, ok := <-input:
					if !ok {
						return nil
					}

					secret, err := n.srv.vault.CreateToken(ctx, alloc, task)
					if err != nil {
						return err
					}

					results[task] = secret
				case <-ctx.Done():
					return nil
				}
			}
		})
	}

	// Send the input
	go func() {
		defer close(input)
		for _, task := range args.Tasks {
			select {
			case <-ctx.Done():
				return
			case input <- task:
			}
		}
	}()

	// Wait for everything to complete or for an error
	createErr := g.Wait()

	// Retrieve the results
	accessors := make([]*structs.VaultAccessor, 0, len(results))
	tokens := make(map[string]string, len(results))
	for task, secret := range results {
		w := secret.WrapInfo
		tokens[task] = w.Token
		accessor := &structs.VaultAccessor{
			Accessor:    w.WrappedAccessor,
			Task:        task,
			NodeID:      alloc.NodeID,
			AllocID:     alloc.ID,
			CreationTTL: w.TTL,
		}

		accessors = append(accessors, accessor)
	}

	// If there was an error revoke the created tokens
	if createErr != nil {
		n.logger.Error("Vault token creation for alloc failed", "alloc_id", alloc.ID, "error", createErr)

		if revokeErr := n.srv.vault.RevokeTokens(context.Background(), accessors, false); revokeErr != nil {
			n.logger.Error("Vault token revocation for alloc failed", "alloc_id", alloc.ID, "error", revokeErr)
		}

		if rerr, ok := createErr.(*structs.RecoverableError); ok {
			reply.Error = rerr
		} else {
			reply.Error = structs.NewRecoverableError(createErr, false).(*structs.RecoverableError)
		}

		return nil
	}

	// Commit to Raft before returning any of the tokens
	req := structs.VaultAccessorsRequest{Accessors: accessors}
	_, index, err := n.srv.raftApply(structs.VaultAccessorRegisterRequestType, &req)
	if err != nil {
		n.logger.Error("registering Vault accessors for alloc failed", "alloc_id", alloc.ID, "error", err)

		// Determine if we can recover from the error
		retry := false
		switch err {
		case raft.ErrNotLeader, raft.ErrLeadershipLost, raft.ErrRaftShutdown, raft.ErrEnqueueTimeout:
			retry = true
		}

		setError(err, retry)
		return nil
	}

	reply.Index = index
	reply.Tasks = tokens
	n.srv.setQueryMeta(&reply.QueryMeta)
	return nil
}

type connectTask struct {
	TaskKind structs.TaskKind
	TaskName string
}

func (n *Node) DeriveSIToken(args *structs.DeriveSITokenRequest, reply *structs.DeriveSITokenResponse) error {

	authErr := n.srv.Authenticate(n.ctx, args)

	setError := func(e error, recoverable bool) {
		if e != nil {
			if re, ok := e.(*structs.RecoverableError); ok {
				reply.Error = re // No need to wrap if error is already a RecoverableError
			} else {
				reply.Error = structs.NewRecoverableError(e, recoverable).(*structs.RecoverableError)
			}
			n.logger.Error("DeriveSIToken failed", "recoverable", recoverable, "error", e)
		}
	}

	if done, err := n.srv.forward("Node.DeriveSIToken", args, args, reply); done {
		setError(err, structs.IsRecoverable(err) || err == structs.ErrNoLeader)
		return nil
	}
	n.srv.MeasureRPCRate("node", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "client", "derive_si_token"}, time.Now())

	// Verify the arguments
	if err := args.Validate(); err != nil {
		setError(err, false)
		return nil
	}

	// Get the ClusterID
	clusterID, err := n.srv.ClusterID()
	if err != nil {
		setError(err, false)
		return nil
	}

	// Verify the following:
	// * The Node exists and has the correct SecretID.
	// * The Allocation exists on the specified Node.
	// * The Allocation contains the given tasks, and each task requires a
	//   SI token.

	snap, err := n.srv.fsm.State().Snapshot()
	if err != nil {
		setError(err, false)
		return nil
	}
	node, err := snap.NodeByID(nil, args.NodeID)
	if err != nil {
		setError(err, false)
		return nil
	}
	if node == nil {
		setError(fmt.Errorf("Node %q does not exist", args.NodeID), false)
		return nil
	}
	if node.SecretID != args.SecretID {
		setError(errors.New("SecretID mismatch"), false)
		return nil
	}

	alloc, err := snap.AllocByID(nil, args.AllocID)
	if err != nil {
		setError(err, false)
		return nil
	}
	if alloc == nil {
		setError(fmt.Errorf("Allocation %q does not exist", args.AllocID), false)
		return nil
	}
	if alloc.NodeID != args.NodeID {
		setError(fmt.Errorf("Allocation %q not running on node %q", args.AllocID, args.NodeID), false)
		return nil
	}
	if alloc.TerminalStatus() {
		setError(errors.New("Cannot request SI token for terminal allocation"), false)
		return nil
	}

	// make sure task group contains at least one connect enabled service
	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
	if tg == nil {
		setError(fmt.Errorf("Allocation %q does not contain TaskGroup %q", args.AllocID, alloc.TaskGroup), false)
		return nil
	}
	if !tg.UsesConnect() {
		setError(fmt.Errorf("TaskGroup %q does not use Connect", tg.Name), false)
		return nil
	}

	// make sure each task in args.Tasks is a connect-enabled task
	notConnect, tasks := connectTasks(tg, args.Tasks)
	if len(notConnect) > 0 {
		setError(fmt.Errorf(
			"Requested Consul Service Identity tokens for tasks that are not Connect enabled: %v",
			strings.Join(notConnect, ", "),
		), false)
	}

	// At this point the request is valid and we should contact Consul for tokens.

	// A lot of the following is copied from DeriveVaultToken which has been
	// working fine for years.

	// Create an error group where we will spin up a fixed set of goroutines to
	// handle deriving tokens but where if any fails the whole group is
	// canceled.
	g, ctx := errgroup.WithContext(context.Background())

	// Cap the worker threads
	numWorkers := len(args.Tasks)
	if numWorkers > maxParallelRequestsPerDerive {
		numWorkers = maxParallelRequestsPerDerive
	}

	// would like to pull some of this out...

	// Create the SI tokens from a slice of task name + connect service
	input := make(chan connectTask, numWorkers)
	results := make(map[string]*structs.SIToken, numWorkers)
	for i := 0; i < numWorkers; i++ {
		g.Go(func() error {
			for {
				select {
				case task, ok := <-input:
					if !ok {
						return nil
					}
					secret, err := n.srv.consulACLs.CreateToken(ctx, ServiceIdentityRequest{
						ConsulNamespace: tg.Consul.GetNamespace(),
						TaskKind:        task.TaskKind,
						TaskName:        task.TaskName,
						ClusterID:       clusterID,
						AllocID:         alloc.ID,
					})
					if err != nil {
						return err
					}
					results[task.TaskName] = secret
				case <-ctx.Done():
					return nil
				}
			}
		})
	}

	// Send the input
	go func() {
		defer close(input)
		for _, connectTask := range tasks {
			select {
			case <-ctx.Done():
				return
			case input <- connectTask:
			}
		}
	}()

	// Wait for everything to complete or for an error
	createErr := g.Wait()

	accessors := make([]*structs.SITokenAccessor, 0, len(results))
	tokens := make(map[string]string, len(results))
	for task, secret := range results {
		tokens[task] = secret.SecretID
		accessor := &structs.SITokenAccessor{
			ConsulNamespace: tg.Consul.GetNamespace(),
			NodeID:          alloc.NodeID,
			AllocID:         alloc.ID,
			TaskName:        task,
			AccessorID:      secret.AccessorID,
		}
		accessors = append(accessors, accessor)
	}

	// If there was an error, revoke all created tokens. These tokens have not
	// yet been committed to the persistent store.
	if createErr != nil {
		n.logger.Error("Consul Service Identity token creation for alloc failed", "alloc_id", alloc.ID, "error", createErr)
		_ = n.srv.consulACLs.RevokeTokens(context.Background(), accessors, false)

		if recoverable, ok := createErr.(*structs.RecoverableError); ok {
			reply.Error = recoverable
		} else {
			reply.Error = structs.NewRecoverableError(createErr, false).(*structs.RecoverableError)
		}

		return nil
	}

	// Commit the derived tokens to raft before returning them
	requested := structs.SITokenAccessorsRequest{Accessors: accessors}
	_, index, err := n.srv.raftApply(structs.ServiceIdentityAccessorRegisterRequestType, &requested)
	if err != nil {
		n.logger.Error("registering Service Identity token accessors for alloc failed", "alloc_id", alloc.ID, "error", err)

		// Determine if we can recover from the error
		retry := false
		switch err {
		case raft.ErrNotLeader, raft.ErrLeadershipLost, raft.ErrRaftShutdown, raft.ErrEnqueueTimeout:
			retry = true
		}
		setError(err, retry)
		return nil
	}

	// We made it! Now we can set the reply.
	reply.Index = index
	reply.Tokens = tokens
	n.srv.setQueryMeta(&reply.QueryMeta)
	return nil
}

func connectTasks(tg *structs.TaskGroup, tasks []string) ([]string, []connectTask) {
	var notConnect []string
	var usesConnect []connectTask
	for _, task := range tasks {
		tgTask := tg.LookupTask(task)
		if !taskUsesConnect(tgTask) {
			notConnect = append(notConnect, task)
		} else {
			usesConnect = append(usesConnect, connectTask{
				TaskName: task,
				TaskKind: tgTask.Kind,
			})
		}
	}
	return notConnect, usesConnect
}

func taskUsesConnect(task *structs.Task) bool {
	if task == nil {
		// not even in the task group
		return false
	}
	return task.UsesConnect()
}

func (n *Node) EmitEvents(args *structs.EmitNodeEventsRequest, reply *structs.EmitNodeEventsResponse) error {

	authErr := n.srv.Authenticate(n.ctx, args)

	// Ensure the connection was initiated by another client if TLS is used.
	err := validateTLSCertificateLevel(n.srv, n.ctx, tlsCertificateLevelClient)
	if err != nil {
		return err
	}
	if done, err := n.srv.forward("Node.EmitEvents", args, args, reply); done {
		return err
	}
	n.srv.MeasureRPCRate("node", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "client", "emit_events"}, time.Now())

	if len(args.NodeEvents) == 0 {
		return fmt.Errorf("no node events given")
	}
	for nodeID, events := range args.NodeEvents {
		if len(events) == 0 {
			return fmt.Errorf("no node events given for node %q", nodeID)
		}
	}

	_, index, err := n.srv.raftApply(structs.UpsertNodeEventsType, args)
	if err != nil {
		n.logger.Error("upserting node events failed", "error", err)
		return err
	}

	reply.Index = index
	return nil
}

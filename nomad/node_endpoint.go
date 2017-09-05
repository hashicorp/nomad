package nomad

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/raft"
	vapi "github.com/hashicorp/vault/api"
)

const (
	// batchUpdateInterval is how long we wait to batch updates
	batchUpdateInterval = 50 * time.Millisecond

	// maxParallelRequestsPerDerive  is the maximum number of parallel Vault
	// create token requests that may be outstanding per derive request
	maxParallelRequestsPerDerive = 16
)

// Node endpoint is used for client interactions
type Node struct {
	srv *Server

	// updates holds pending client status updates for allocations
	updates []*structs.Allocation

	// updateFuture is used to wait for the pending batch update
	// to complete. This may be nil if no batch is pending.
	updateFuture *batchFuture

	// updateTimer is the timer that will trigger the next batch
	// update, and may be nil if there is no batch pending.
	updateTimer *time.Timer

	// updatesLock synchronizes access to the updates list,
	// the future and the timer.
	updatesLock sync.Mutex
}

// Register is used to upsert a client that is available for scheduling
func (n *Node) Register(args *structs.NodeRegisterRequest, reply *structs.NodeUpdateResponse) error {
	if done, err := n.srv.forward("Node.Register", args, args, reply); done {
		return err
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

	// COMPAT: Remove after 0.6
	// Need to check if this node is <0.4.x since SecretID is new in 0.5
	pre, err := nodePreSecretID(args.Node)
	if err != nil {
		return err
	}
	if args.Node.SecretID == "" && !pre {
		return fmt.Errorf("missing node secret ID for client registration")
	}

	// Default the status if none is given
	if args.Node.Status == "" {
		args.Node.Status = structs.NodeStatusInit
	}
	if !structs.ValidNodeStatus(args.Node.Status) {
		return fmt.Errorf("invalid status for node")
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

	// Check if the SecretID has been tampered with
	if !pre && originalNode != nil {
		if args.Node.SecretID != originalNode.SecretID && originalNode.SecretID != "" {
			return fmt.Errorf("node secret ID does not match. Not registering node.")
		}
	}

	// Commit this update via Raft
	_, index, err := n.srv.raftApply(structs.NodeRegisterRequestType, args)
	if err != nil {
		n.srv.logger.Printf("[ERR] nomad.client: Register failed: %v", err)
		return err
	}
	reply.NodeModifyIndex = index

	// Check if we should trigger evaluations
	originalStatus := structs.NodeStatusInit
	if originalNode != nil {
		originalStatus = originalNode.Status
	}
	transitionToReady := transitionedToReady(args.Node.Status, originalStatus)
	if structs.ShouldDrainNode(args.Node.Status) || transitionToReady {
		evalIDs, evalIndex, err := n.createNodeEvals(args.Node.ID, index)
		if err != nil {
			n.srv.logger.Printf("[ERR] nomad.client: eval creation failed: %v", err)
			return err
		}
		reply.EvalIDs = evalIDs
		reply.EvalCreateIndex = evalIndex
	}

	// Check if we need to setup a heartbeat
	if !args.Node.TerminalStatus() {
		ttl, err := n.srv.resetHeartbeatTimer(args.Node.ID)
		if err != nil {
			n.srv.logger.Printf("[ERR] nomad.client: heartbeat reset failed: %v", err)
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
	if err := n.constructNodeServerInfoResponse(snap, reply); err != nil {
		n.srv.logger.Printf("[ERR] nomad.client: failed to populate NodeUpdateResponse: %v", err)
		return err
	}

	return nil
}

// nodePreSecretID is a helper that returns whether the node is on a version
// that is before SecretIDs were introduced
func nodePreSecretID(node *structs.Node) (bool, error) {
	a := node.Attributes
	if a == nil {
		return false, fmt.Errorf("node doesn't have attributes set")
	}

	v, ok := a["nomad.version"]
	if !ok {
		return false, fmt.Errorf("missing Nomad version in attributes")
	}

	return !strings.HasPrefix(v, "0.5"), nil
}

// updateNodeUpdateResponse assumes the n.srv.peerLock is held for reading.
func (n *Node) constructNodeServerInfoResponse(snap *state.StateSnapshot, reply *structs.NodeUpdateResponse) error {
	reply.LeaderRPCAddr = string(n.srv.raft.Leader())

	// Reply with config information required for future RPC requests
	reply.Servers = make([]*structs.NodeServerInfo, 0, len(n.srv.localPeers))
	for k, v := range n.srv.localPeers {
		reply.Servers = append(reply.Servers,
			&structs.NodeServerInfo{
				RPCAdvertiseAddr: string(k),
				RPCMajorVersion:  int32(v.MajorVersion),
				RPCMinorVersion:  int32(v.MinorVersion),
				Datacenter:       v.Datacenter,
			})
	}

	// TODO(sean@): Use an indexed node count instead
	//
	// Snapshot is used only to iterate over all nodes to create a node
	// count to send back to Nomad Clients in their heartbeat so Clients
	// can estimate the size of the cluster.
	ws := memdb.NewWatchSet()
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

	return nil
}

// Deregister is used to remove a client from the cluster. If a client should
// just be made unavailable for scheduling, a status update is preferred.
func (n *Node) Deregister(args *structs.NodeDeregisterRequest, reply *structs.NodeUpdateResponse) error {
	if done, err := n.srv.forward("Node.Deregister", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "client", "deregister"}, time.Now())

	// Verify the arguments
	if args.NodeID == "" {
		return fmt.Errorf("missing node ID for client deregistration")
	}

	// Commit this update via Raft
	_, index, err := n.srv.raftApply(structs.NodeDeregisterRequestType, args)
	if err != nil {
		n.srv.logger.Printf("[ERR] nomad.client: Deregister failed: %v", err)
		return err
	}

	// Clear the heartbeat timer if any
	n.srv.clearHeartbeatTimer(args.NodeID)

	// Create the evaluations for this node
	evalIDs, evalIndex, err := n.createNodeEvals(args.NodeID, index)
	if err != nil {
		n.srv.logger.Printf("[ERR] nomad.client: eval creation failed: %v", err)
		return err
	}

	// Determine if there are any Vault accessors on the node
	ws := memdb.NewWatchSet()
	accessors, err := n.srv.State().VaultAccessorsByNode(ws, args.NodeID)
	if err != nil {
		n.srv.logger.Printf("[ERR] nomad.client: looking up accessors for node %q failed: %v", args.NodeID, err)
		return err
	}

	if l := len(accessors); l != 0 {
		n.srv.logger.Printf("[DEBUG] nomad.client: revoking %d accessors on node %q due to deregister", l, args.NodeID)
		if err := n.srv.vault.RevokeTokens(context.Background(), accessors, true); err != nil {
			n.srv.logger.Printf("[ERR] nomad.client: revoking accessors for node %q failed: %v", args.NodeID, err)
			return err
		}
	}

	// Setup the reply
	reply.EvalIDs = evalIDs
	reply.EvalCreateIndex = evalIndex
	reply.NodeModifyIndex = index
	reply.Index = index
	return nil
}

// UpdateStatus is used to update the status of a client node
func (n *Node) UpdateStatus(args *structs.NodeUpdateStatusRequest, reply *structs.NodeUpdateResponse) error {
	if done, err := n.srv.forward("Node.UpdateStatus", args, args, reply); done {
		return err
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

	// XXX: Could use the SecretID here but have to update the heartbeat system
	// to track SecretIDs.

	// Update the timestamp of when the node status was updated
	node.StatusUpdatedAt = time.Now().Unix()

	// Commit this update via Raft
	var index uint64
	if node.Status != args.Status {
		_, index, err = n.srv.raftApply(structs.NodeUpdateStatusRequestType, args)
		if err != nil {
			n.srv.logger.Printf("[ERR] nomad.client: status update failed: %v", err)
			return err
		}
		reply.NodeModifyIndex = index
	}

	// Check if we should trigger evaluations
	transitionToReady := transitionedToReady(args.Status, node.Status)
	if structs.ShouldDrainNode(args.Status) || transitionToReady {
		evalIDs, evalIndex, err := n.createNodeEvals(args.NodeID, index)
		if err != nil {
			n.srv.logger.Printf("[ERR] nomad.client: eval creation failed: %v", err)
			return err
		}
		reply.EvalIDs = evalIDs
		reply.EvalCreateIndex = evalIndex
	}

	// Check if we need to setup a heartbeat
	switch args.Status {
	case structs.NodeStatusDown:
		// Determine if there are any Vault accessors on the node
		accessors, err := n.srv.State().VaultAccessorsByNode(ws, args.NodeID)
		if err != nil {
			n.srv.logger.Printf("[ERR] nomad.client: looking up accessors for node %q failed: %v", args.NodeID, err)
			return err
		}

		if l := len(accessors); l != 0 {
			n.srv.logger.Printf("[DEBUG] nomad.client: revoking %d accessors on node %q due to down state", l, args.NodeID)
			if err := n.srv.vault.RevokeTokens(context.Background(), accessors, true); err != nil {
				n.srv.logger.Printf("[ERR] nomad.client: revoking accessors for node %q failed: %v", args.NodeID, err)
				return err
			}
		}
	default:
		ttl, err := n.srv.resetHeartbeatTimer(args.NodeID)
		if err != nil {
			n.srv.logger.Printf("[ERR] nomad.client: heartbeat reset failed: %v", err)
			return err
		}
		reply.HeartbeatTTL = ttl
	}

	// Set the reply index and leader
	reply.Index = index
	n.srv.peerLock.RLock()
	defer n.srv.peerLock.RUnlock()
	if err := n.constructNodeServerInfoResponse(snap, reply); err != nil {
		n.srv.logger.Printf("[ERR] nomad.client: failed to populate NodeUpdateResponse: %v", err)
		return err
	}

	return nil
}

// transitionedToReady is a helper that takes a nodes new and old status and
// returns whether it has transistioned to ready.
func transitionedToReady(newStatus, oldStatus string) bool {
	initToReady := oldStatus == structs.NodeStatusInit && newStatus == structs.NodeStatusReady
	terminalToReady := oldStatus == structs.NodeStatusDown && newStatus == structs.NodeStatusReady
	return initToReady || terminalToReady
}

// UpdateDrain is used to update the drain mode of a client node
func (n *Node) UpdateDrain(args *structs.NodeUpdateDrainRequest,
	reply *structs.NodeDrainUpdateResponse) error {
	if done, err := n.srv.forward("Node.UpdateDrain", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "client", "update_drain"}, time.Now())

	// Verify the arguments
	if args.NodeID == "" {
		return fmt.Errorf("missing node ID for drain update")
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

	// Update the timestamp to
	node.StatusUpdatedAt = time.Now().Unix()

	// Commit this update via Raft
	var index uint64
	if node.Drain != args.Drain {
		_, index, err = n.srv.raftApply(structs.NodeUpdateDrainRequestType, args)
		if err != nil {
			n.srv.logger.Printf("[ERR] nomad.client: drain update failed: %v", err)
			return err
		}
		reply.NodeModifyIndex = index
	}

	// Always attempt to create Node evaluations because there may be a System
	// job registered that should be evaluated.
	evalIDs, evalIndex, err := n.createNodeEvals(args.NodeID, index)
	if err != nil {
		n.srv.logger.Printf("[ERR] nomad.client: eval creation failed: %v", err)
		return err
	}
	reply.EvalIDs = evalIDs
	reply.EvalCreateIndex = evalIndex

	// Set the reply index
	reply.Index = index
	return nil
}

// Evaluate is used to force a re-evaluation of the node
func (n *Node) Evaluate(args *structs.NodeEvaluateRequest, reply *structs.NodeUpdateResponse) error {
	if done, err := n.srv.forward("Node.Evaluate", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "client", "evaluate"}, time.Now())

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
	evalIDs, evalIndex, err := n.createNodeEvals(args.NodeID, node.ModifyIndex)
	if err != nil {
		n.srv.logger.Printf("[ERR] nomad.client: eval creation failed: %v", err)
		return err
	}
	reply.EvalIDs = evalIDs
	reply.EvalCreateIndex = evalIndex

	// Set the reply index
	reply.Index = evalIndex

	n.srv.peerLock.RLock()
	defer n.srv.peerLock.RUnlock()
	if err := n.constructNodeServerInfoResponse(snap, reply); err != nil {
		n.srv.logger.Printf("[ERR] nomad.client: failed to populate NodeUpdateResponse: %v", err)
		return err
	}
	return nil
}

// GetNode is used to request information about a specific node
func (n *Node) GetNode(args *structs.NodeSpecificRequest,
	reply *structs.SingleNodeResponse) error {
	if done, err := n.srv.forward("Node.GetNode", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "client", "get_node"}, time.Now())

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
				// Clear the secret ID
				reply.Node = out.Copy()
				reply.Node.SecretID = ""
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
	if done, err := n.srv.forward("Node.GetAllocs", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "client", "get_allocs"}, time.Now())

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
			if len(allocs) != 0 {
				reply.Allocs = allocs
				for _, alloc := range allocs {
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
	if done, err := n.srv.forward("Node.GetClientAllocs", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "client", "get_client_allocs"}, time.Now())

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
			node, err := state.NodeByID(ws, args.NodeID)
			if err != nil {
				return err
			}

			var allocs []*structs.Allocation
			if node != nil {
				// COMPAT: Remove in 0.6
				// Check if the node should have a SecretID set
				if args.SecretID == "" {
					if pre, err := nodePreSecretID(node); err != nil {
						return err
					} else if !pre {
						return fmt.Errorf("missing node secret ID for client status update")
					}
				} else if args.SecretID != node.SecretID {
					return fmt.Errorf("node secret ID does not match")
				}

				var err error
				allocs, err = state.AllocsByNode(ws, args.NodeID)
				if err != nil {
					return err
				}
			}

			reply.Allocs = make(map[string]uint64)
			// Setup the output
			if len(allocs) != 0 {
				for _, alloc := range allocs {
					reply.Allocs[alloc.ID] = alloc.AllocModifyIndex
					reply.Index = maxUint64(reply.Index, alloc.ModifyIndex)
				}
			} else {
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

// UpdateAlloc is used to update the client status of an allocation
func (n *Node) UpdateAlloc(args *structs.AllocUpdateRequest, reply *structs.GenericResponse) error {
	if done, err := n.srv.forward("Node.UpdateAlloc", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "client", "update_alloc"}, time.Now())

	// Ensure at least a single alloc
	if len(args.Alloc) == 0 {
		return fmt.Errorf("must update at least one allocation")
	}

	// Add this to the batch
	n.updatesLock.Lock()
	n.updates = append(n.updates, args.Alloc...)

	// Start a new batch if none
	future := n.updateFuture
	if future == nil {
		future = NewBatchFuture()
		n.updateFuture = future
		n.updateTimer = time.AfterFunc(batchUpdateInterval, func() {
			// Get the pending updates
			n.updatesLock.Lock()
			updates := n.updates
			future := n.updateFuture
			n.updates = nil
			n.updateFuture = nil
			n.updateTimer = nil
			n.updatesLock.Unlock()

			// Perform the batch update
			n.batchUpdate(future, updates)
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
func (n *Node) batchUpdate(future *batchFuture, updates []*structs.Allocation) {
	// Prepare the batch update
	batch := &structs.AllocUpdateRequest{
		Alloc:        updates,
		WriteRequest: structs.WriteRequest{Region: n.srv.config.Region},
	}

	// Commit this update via Raft
	var mErr multierror.Error
	_, index, err := n.srv.raftApply(structs.AllocClientUpdateRequestType, batch)
	if err != nil {
		n.srv.logger.Printf("[ERR] nomad.client: alloc update failed: %v", err)
		mErr.Errors = append(mErr.Errors, err)
	}

	// For each allocation we are updating check if we should revoke any
	// Vault Accessors
	var revoke []*structs.VaultAccessor
	for _, alloc := range updates {
		// Skip any allocation that isn't dead on the client
		if !alloc.Terminated() {
			continue
		}

		// Determine if there are any Vault accessors for the allocation
		ws := memdb.NewWatchSet()
		accessors, err := n.srv.State().VaultAccessorsByAlloc(ws, alloc.ID)
		if err != nil {
			n.srv.logger.Printf("[ERR] nomad.client: looking up accessors for alloc %q failed: %v", alloc.ID, err)
			mErr.Errors = append(mErr.Errors, err)
		}

		revoke = append(revoke, accessors...)
	}

	if l := len(revoke); l != 0 {
		n.srv.logger.Printf("[DEBUG] nomad.client: revoking %d accessors due to terminal allocations", l)
		if err := n.srv.vault.RevokeTokens(context.Background(), revoke, true); err != nil {
			n.srv.logger.Printf("[ERR] nomad.client: batched accessor revocation failed: %v", err)
			mErr.Errors = append(mErr.Errors, err)
		}
	}

	// Respond to the future
	future.Respond(index, mErr.ErrorOrNil())
}

// List is used to list the available nodes
func (n *Node) List(args *structs.NodeListRequest,
	reply *structs.NodeListResponse) error {
	if done, err := n.srv.forward("Node.List", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "client", "list"}, time.Now())

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			// Capture all the nodes
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

			var nodes []*structs.NodeListStub
			for {
				raw := iter.Next()
				if raw == nil {
					break
				}
				node := raw.(*structs.Node)
				nodes = append(nodes, node.Stub())
			}
			reply.Nodes = nodes

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
func (n *Node) createNodeEvals(nodeID string, nodeIndex uint64) ([]string, uint64, error) {
	// Snapshot the state
	snap, err := n.srv.fsm.State().Snapshot()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to snapshot state: %v", err)
	}

	// Find all the allocations for this node
	ws := memdb.NewWatchSet()
	allocs, err := snap.AllocsByNode(ws, nodeID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to find allocs for '%s': %v", nodeID, err)
	}

	sysJobsIter, err := snap.JobsByScheduler(ws, "system")
	if err != nil {
		return nil, 0, fmt.Errorf("failed to find system jobs for '%s': %v", nodeID, err)
	}

	var sysJobs []*structs.Job
	for job := sysJobsIter.Next(); job != nil; job = sysJobsIter.Next() {
		sysJobs = append(sysJobs, job.(*structs.Job))
	}

	// Fast-path if nothing to do
	if len(allocs) == 0 && len(sysJobs) == 0 {
		return nil, 0, nil
	}

	// Create an eval for each JobID affected
	var evals []*structs.Evaluation
	var evalIDs []string
	jobIDs := make(map[string]struct{})

	for _, alloc := range allocs {
		// Deduplicate on JobID
		if _, ok := jobIDs[alloc.JobID]; ok {
			continue
		}
		jobIDs[alloc.JobID] = struct{}{}

		// Create a new eval
		eval := &structs.Evaluation{
			ID:              structs.GenerateUUID(),
			Namespace:       alloc.Namespace,
			Priority:        alloc.Job.Priority,
			Type:            alloc.Job.Type,
			TriggeredBy:     structs.EvalTriggerNodeUpdate,
			JobID:           alloc.JobID,
			NodeID:          nodeID,
			NodeModifyIndex: nodeIndex,
			Status:          structs.EvalStatusPending,
		}
		evals = append(evals, eval)
		evalIDs = append(evalIDs, eval.ID)
	}

	// Create an evaluation for each system job.
	for _, job := range sysJobs {
		// Still dedup on JobID as the node may already have the system job.
		if _, ok := jobIDs[job.ID]; ok {
			continue
		}
		jobIDs[job.ID] = struct{}{}

		// Create a new eval
		eval := &structs.Evaluation{
			ID:              structs.GenerateUUID(),
			Namespace:       job.Namespace,
			Priority:        job.Priority,
			Type:            job.Type,
			TriggeredBy:     structs.EvalTriggerNodeUpdate,
			JobID:           job.ID,
			NodeID:          nodeID,
			NodeModifyIndex: nodeIndex,
			Status:          structs.EvalStatusPending,
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

// batchFuture is used to wait on a batch update to complete
type batchFuture struct {
	doneCh chan struct{}
	err    error
	index  uint64
}

// NewBatchFuture creates a new batch future
func NewBatchFuture() *batchFuture {
	return &batchFuture{
		doneCh: make(chan struct{}),
	}
}

// Wait is used to block for the future to complete and returns the error
func (b *batchFuture) Wait() error {
	<-b.doneCh
	return b.err
}

// Index is used to return the index of the batch, only after Wait()
func (b *batchFuture) Index() uint64 {
	return b.index
}

// Respond is used to unblock the future
func (b *batchFuture) Respond(index uint64, err error) {
	b.index = index
	b.err = err
	close(b.doneCh)
}

// DeriveVaultToken is used by the clients to request wrapped Vault tokens for
// tasks
func (n *Node) DeriveVaultToken(args *structs.DeriveVaultTokenRequest,
	reply *structs.DeriveVaultTokenResponse) error {

	// setErr is a helper for setting the recoverable error on the reply and
	// logging it
	setErr := func(e error, recoverable bool) {
		if e == nil {
			return
		}
		reply.Error = structs.NewRecoverableError(e, recoverable).(*structs.RecoverableError)
		n.srv.logger.Printf("[ERR] nomad.client: DeriveVaultToken failed (recoverable %v): %v", recoverable, e)
	}

	if done, err := n.srv.forward("Node.DeriveVaultToken", args, args, reply); done {
		setErr(err, structs.IsRecoverable(err) || err == structs.ErrNoLeader)
		return nil
	}
	defer metrics.MeasureSince([]string{"nomad", "client", "derive_vault_token"}, time.Now())

	// Verify the arguments
	if args.NodeID == "" {
		setErr(fmt.Errorf("missing node ID"), false)
		return nil
	}
	if args.SecretID == "" {
		setErr(fmt.Errorf("missing node SecretID"), false)
		return nil
	}
	if args.AllocID == "" {
		setErr(fmt.Errorf("missing allocation ID"), false)
		return nil
	}
	if len(args.Tasks) == 0 {
		setErr(fmt.Errorf("no tasks specified"), false)
		return nil
	}

	// Verify the following:
	// * The Node exists and has the correct SecretID
	// * The Allocation exists on the specified node
	// * The allocation contains the given tasks and they each require Vault
	//   tokens
	snap, err := n.srv.fsm.State().Snapshot()
	if err != nil {
		setErr(err, false)
		return nil
	}
	ws := memdb.NewWatchSet()
	node, err := snap.NodeByID(ws, args.NodeID)
	if err != nil {
		setErr(err, false)
		return nil
	}
	if node == nil {
		setErr(fmt.Errorf("Node %q does not exist", args.NodeID), false)
		return nil
	}
	if node.SecretID != args.SecretID {
		setErr(fmt.Errorf("SecretID mismatch"), false)
		return nil
	}

	alloc, err := snap.AllocByID(ws, args.AllocID)
	if err != nil {
		setErr(err, false)
		return nil
	}
	if alloc == nil {
		setErr(fmt.Errorf("Allocation %q does not exist", args.AllocID), false)
		return nil
	}
	if alloc.NodeID != args.NodeID {
		setErr(fmt.Errorf("Allocation %q not running on Node %q", args.AllocID, args.NodeID), false)
		return nil
	}
	if alloc.TerminalStatus() {
		setErr(fmt.Errorf("Can't request Vault token for terminal allocation"), false)
		return nil
	}

	// Check the policies
	policies := alloc.Job.VaultPolicies()
	if policies == nil {
		setErr(fmt.Errorf("Job doesn't require Vault policies"), false)
		return nil
	}
	tg, ok := policies[alloc.TaskGroup]
	if !ok {
		setErr(fmt.Errorf("Task group does not require Vault policies"), false)
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
		setErr(e, false)
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
						wrapped := fmt.Sprintf("failed to create token for task %q on alloc %q: %v", task, alloc.ID, err)
						return structs.WrapRecoverable(wrapped, err)
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
		if w == nil {
			return fmt.Errorf("Vault returned Secret without WrapInfo")
		}

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
		n.srv.logger.Printf("[ERR] nomad.node: Vault token creation for alloc %q failed: %v", alloc.ID, createErr)

		if revokeErr := n.srv.vault.RevokeTokens(context.Background(), accessors, false); revokeErr != nil {
			n.srv.logger.Printf("[ERR] nomad.node: Vault token revocation for alloc %q failed: %v", alloc.ID, revokeErr)
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
		n.srv.logger.Printf("[ERR] nomad.client: Register Vault accessors for alloc %q failed: %v", alloc.ID, err)

		// Determine if we can recover from the error
		retry := false
		switch err {
		case raft.ErrNotLeader, raft.ErrLeadershipLost, raft.ErrRaftShutdown, raft.ErrEnqueueTimeout:
			retry = true
		}

		setErr(err, retry)
		return nil
	}

	reply.Index = index
	reply.Tasks = tokens
	n.srv.setQueryMeta(&reply.QueryMeta)
	return nil
}

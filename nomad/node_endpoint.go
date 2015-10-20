package nomad

import (
	"fmt"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/nomad/nomad/structs"
)

// Node endpoint is used for client interactions
type Node struct {
	srv *Server
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

	// Default the status if none is given
	if args.Node.Status == "" {
		args.Node.Status = structs.NodeStatusInit
	}
	if !structs.ValidNodeStatus(args.Node.Status) {
		return fmt.Errorf("invalid status for node")
	}

	// Commit this update via Raft
	_, index, err := n.srv.raftApply(structs.NodeRegisterRequestType, args)
	if err != nil {
		n.srv.logger.Printf("[ERR] nomad.client: Register failed: %v", err)
		return err
	}
	reply.NodeModifyIndex = index

	// Check if we should trigger evaluations
	if structs.ShouldDrainNode(args.Node.Status) {
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
	return nil
}

// Deregister is used to remove a client from the client. If a client should
// just be made unavailable for scheduling, a status update is prefered.
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
		return fmt.Errorf("missing node ID for client deregistration")
	}
	if !structs.ValidNodeStatus(args.Status) {
		return fmt.Errorf("invalid status for node")
	}

	// Look for the node
	snap, err := n.srv.fsm.State().Snapshot()
	if err != nil {
		return err
	}
	node, err := snap.NodeByID(args.NodeID)
	if err != nil {
		return err
	}
	if node == nil {
		return fmt.Errorf("node not found")
	}

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
	initToReady := node.Status == structs.NodeStatusInit && args.Status == structs.NodeStatusReady
	if structs.ShouldDrainNode(args.Status) || initToReady {
		evalIDs, evalIndex, err := n.createNodeEvals(args.NodeID, index)
		if err != nil {
			n.srv.logger.Printf("[ERR] nomad.client: eval creation failed: %v", err)
			return err
		}
		reply.EvalIDs = evalIDs
		reply.EvalCreateIndex = evalIndex
	}

	// Check if we need to setup a heartbeat
	if args.Status != structs.NodeStatusDown {
		ttl, err := n.srv.resetHeartbeatTimer(args.NodeID)
		if err != nil {
			n.srv.logger.Printf("[ERR] nomad.client: heartbeat reset failed: %v", err)
			return err
		}
		reply.HeartbeatTTL = ttl
	}

	// Set the reply index
	reply.Index = index
	return nil
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
	node, err := snap.NodeByID(args.NodeID)
	if err != nil {
		return err
	}
	if node == nil {
		return fmt.Errorf("node not found")
	}

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

	// Check if we should trigger evaluations
	if args.Drain {
		evalIDs, evalIndex, err := n.createNodeEvals(args.NodeID, index)
		if err != nil {
			n.srv.logger.Printf("[ERR] nomad.client: eval creation failed: %v", err)
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
	node, err := snap.NodeByID(args.NodeID)
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
	return nil
}

// GetNode is used to request information about a specific node
func (n *Node) GetNode(args *structs.NodeSpecificRequest,
	reply *structs.SingleNodeResponse) error {
	if done, err := n.srv.forward("Node.GetNode", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "client", "get_node"}, time.Now())

	// Verify the arguments
	if args.NodeID == "" {
		return fmt.Errorf("missing node ID")
	}

	// Look for the node
	snap, err := n.srv.fsm.State().Snapshot()
	if err != nil {
		return err
	}
	out, err := snap.NodeByID(args.NodeID)
	if err != nil {
		return err
	}

	// Setup the output
	if out != nil {
		reply.Node = out
		reply.Index = out.ModifyIndex
	} else {
		// Use the last index that affected the nodes table
		index, err := snap.Index("nodes")
		if err != nil {
			return err
		}
		reply.Index = index
	}

	// Set the query response
	n.srv.setQueryMeta(&reply.QueryMeta)
	return nil
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
		queryOpts:  &args.QueryOptions,
		queryMeta:  &reply.QueryMeta,
		allocWatch: args.NodeID,
		run: func() error {
			// Look for the node
			snap, err := n.srv.fsm.State().Snapshot()
			if err != nil {
				return err
			}
			allocs, err := snap.AllocsByNode(args.NodeID)
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
				index, err := snap.Index("allocs")
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

	// Ensure only a single alloc
	if len(args.Alloc) != 1 {
		return fmt.Errorf("must update a single allocation")
	}

	// Commit this update via Raft
	_, index, err := n.srv.raftApply(structs.AllocClientUpdateRequestType, args)
	if err != nil {
		n.srv.logger.Printf("[ERR] nomad.client: alloc update failed: %v", err)
		return err
	}

	// Setup the response
	reply.Index = index
	return nil
}

// List is used to list the available nodes
func (n *Node) List(args *structs.NodeListRequest,
	reply *structs.NodeListResponse) error {
	if done, err := n.srv.forward("Node.List", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "client", "list"}, time.Now())

	// Capture all the nodes
	snap, err := n.srv.fsm.State().Snapshot()
	if err != nil {
		return err
	}
	iter, err := snap.Nodes()
	if err != nil {
		return err
	}

	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		node := raw.(*structs.Node)
		reply.Nodes = append(reply.Nodes, node.Stub())
	}

	// Use the last index that affected the jobs table
	index, err := snap.Index("nodes")
	if err != nil {
		return err
	}
	reply.Index = index

	// Set the query response
	n.srv.setQueryMeta(&reply.QueryMeta)
	return nil
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
	allocs, err := snap.AllocsByNode(nodeID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to find allocs for '%s': %v", nodeID, err)
	}

	sysJobs, err := snap.JobsByScheduler("system")
	if err != nil {
		return nil, 0, fmt.Errorf("failed to find system jobs for '%s': %v", nodeID, err)
	}
	nextJob := sysJobs.Next()

	// Fast-path if nothing to do
	if len(allocs) == 0 && nextJob == nil {
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
	curSysJob := nextJob.(*structs.Job)
	for job := sysJobs.Next(); ; {
		// Still dedup on JobID as the node may already have the system job.
		if _, ok := jobIDs[curSysJob.ID]; ok {
			continue
		}
		jobIDs[curSysJob.ID] = struct{}{}

		// Create a new eval
		eval := &structs.Evaluation{
			ID:              structs.GenerateUUID(),
			Priority:        curSysJob.Priority,
			Type:            curSysJob.Type,
			TriggeredBy:     structs.EvalTriggerNodeUpdate,
			JobID:           curSysJob.ID,
			NodeID:          nodeID,
			NodeModifyIndex: nodeIndex,
			Status:          structs.EvalStatusPending,
		}
		evals = append(evals, eval)
		evalIDs = append(evalIDs, eval.ID)

		// Update the current system job for the next iteration.
		if job == nil {
			break
		}

		curSysJob = job.(*structs.Job)
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

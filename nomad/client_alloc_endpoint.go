package nomad

import (
	"errors"
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/nomad/acl"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
)

// ClientAllocations is used to forward RPC requests to the targed Nomad client's
// Allocation endpoint.
type ClientAllocations struct {
	srv *Server
}

// GarbageCollectAll is used to garbage collect all allocations on a client.
func (a *ClientAllocations) GarbageCollectAll(args *structs.NodeSpecificRequest, reply *structs.GenericResponse) error {
	// We only allow stale reads since the only potentially stale information is
	// the Node registration and the cost is fairly high for adding another hope
	// in the forwarding chain.
	args.QueryOptions.AllowStale = true

	// Potentially forward to a different region.
	if done, err := a.srv.forward("ClientAllocations.GarbageCollectAll", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "client_allocations", "garbage_collect_all"}, time.Now())

	// Check node read permissions
	if aclObj, err := a.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowNodeWrite() {
		return structs.ErrPermissionDenied
	}

	// Verify the arguments.
	if args.NodeID == "" {
		return errors.New("missing NodeID")
	}

	// Make sure Node is valid and new enough to support RPC
	snap, err := a.srv.State().Snapshot()
	if err != nil {
		return err
	}

	_, err = getNodeForRpc(snap, args.NodeID)
	if err != nil {
		return err
	}

	// Get the connection to the client
	state, ok := a.srv.getNodeConn(args.NodeID)
	if !ok {
		return findNodeConnAndForward(a.srv, args.NodeID, "ClientAllocations.GarbageCollectAll", args, reply)
	}

	// Make the RPC
	return NodeRpc(state.Session, "Allocations.GarbageCollectAll", args, reply)
}

// GarbageCollect is used to garbage collect an allocation on a client.
func (a *ClientAllocations) GarbageCollect(args *structs.AllocSpecificRequest, reply *structs.GenericResponse) error {
	// We only allow stale reads since the only potentially stale information is
	// the Node registration and the cost is fairly high for adding another hope
	// in the forwarding chain.
	args.QueryOptions.AllowStale = true

	// Potentially forward to a different region.
	if done, err := a.srv.forward("ClientAllocations.GarbageCollect", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "client_allocations", "garbage_collect"}, time.Now())

	// Check node read permissions
	if aclObj, err := a.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowNsOp(args.Namespace, acl.NamespaceCapabilitySubmitJob) {
		return structs.ErrPermissionDenied
	}

	// Verify the arguments.
	if args.AllocID == "" {
		return errors.New("missing AllocID")
	}

	// Find the allocation
	snap, err := a.srv.State().Snapshot()
	if err != nil {
		return err
	}

	alloc, err := snap.AllocByID(nil, args.AllocID)
	if err != nil {
		return err
	}

	if alloc == nil {
		return structs.NewErrUnknownAllocation(args.AllocID)
	}

	// Make sure Node is valid and new enough to support RPC
	_, err = getNodeForRpc(snap, alloc.NodeID)
	if err != nil {
		return err
	}

	// Get the connection to the client
	state, ok := a.srv.getNodeConn(alloc.NodeID)
	if !ok {
		return findNodeConnAndForward(a.srv, alloc.NodeID, "ClientAllocations.GarbageCollect", args, reply)
	}

	// Make the RPC
	return NodeRpc(state.Session, "Allocations.GarbageCollect", args, reply)
}

// Stats is used to collect allocation statistics
func (a *ClientAllocations) Stats(args *cstructs.AllocStatsRequest, reply *cstructs.AllocStatsResponse) error {
	// We only allow stale reads since the only potentially stale information is
	// the Node registration and the cost is fairly high for adding another hope
	// in the forwarding chain.
	args.QueryOptions.AllowStale = true

	// Potentially forward to a different region.
	if done, err := a.srv.forward("ClientAllocations.Stats", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "client_allocations", "stats"}, time.Now())

	// Check node read permissions
	if aclObj, err := a.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowNsOp(args.Namespace, acl.NamespaceCapabilityReadJob) {
		return structs.ErrPermissionDenied
	}

	// Verify the arguments.
	if args.AllocID == "" {
		return errors.New("missing AllocID")
	}

	// Find the allocation
	snap, err := a.srv.State().Snapshot()
	if err != nil {
		return err
	}

	alloc, err := snap.AllocByID(nil, args.AllocID)
	if err != nil {
		return err
	}

	if alloc == nil {
		return structs.NewErrUnknownAllocation(args.AllocID)
	}

	// Make sure Node is valid and new enough to support RPC
	_, err = getNodeForRpc(snap, alloc.NodeID)
	if err != nil {
		return err
	}

	// Get the connection to the client
	state, ok := a.srv.getNodeConn(alloc.NodeID)
	if !ok {
		return findNodeConnAndForward(a.srv, alloc.NodeID, "ClientAllocations.Stats", args, reply)
	}

	// Make the RPC
	return NodeRpc(state.Session, "Allocations.Stats", args, reply)
}

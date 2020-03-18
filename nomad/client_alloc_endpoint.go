package nomad

import (
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	metrics "github.com/armon/go-metrics"
	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-msgpack/codec"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper"

	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/nomad/structs"
)

// ClientAllocations is used to forward RPC requests to the targed Nomad client's
// Allocation endpoint.
type ClientAllocations struct {
	srv    *Server
	logger log.Logger
}

func (a *ClientAllocations) register() {
	a.srv.streamingRpcs.Register("Allocations.Exec", a.exec)
}

// GarbageCollectAll is used to garbage collect all allocations on a client.
func (a *ClientAllocations) GarbageCollectAll(args *structs.NodeSpecificRequest, reply *structs.GenericResponse) error {
	// We only allow stale reads since the only potentially stale information is
	// the Node registration and the cost is fairly high for adding another hop
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

// Signal is used to send a signal to an allocation on a client.
func (a *ClientAllocations) Signal(args *structs.AllocSignalRequest, reply *structs.GenericResponse) error {
	// We only allow stale reads since the only potentially stale information is
	// the Node registration and the cost is fairly high for adding another hope
	// in the forwarding chain.
	args.QueryOptions.AllowStale = true

	// Potentially forward to a different region.
	if done, err := a.srv.forward("ClientAllocations.Signal", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "client_allocations", "signal"}, time.Now())

	// Verify the arguments.
	if args.AllocID == "" {
		return errors.New("missing AllocID")
	}

	// Find the allocation
	snap, err := a.srv.State().Snapshot()
	if err != nil {
		return err
	}

	alloc, err := getAlloc(snap, args.AllocID)
	if err != nil {
		return err
	}

	// Check namespace alloc-lifecycle permission.
	if aclObj, err := a.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowNsOp(alloc.Namespace, acl.NamespaceCapabilityAllocLifecycle) {
		return structs.ErrPermissionDenied
	}

	// Make sure Node is valid and new enough to support RPC
	_, err = getNodeForRpc(snap, alloc.NodeID)
	if err != nil {
		return err
	}

	// Get the connection to the client
	state, ok := a.srv.getNodeConn(alloc.NodeID)
	if !ok {
		return findNodeConnAndForward(a.srv, alloc.NodeID, "ClientAllocations.Signal", args, reply)
	}

	// Make the RPC
	return NodeRpc(state.Session, "Allocations.Signal", args, reply)
}

// GarbageCollect is used to garbage collect an allocation on a client.
func (a *ClientAllocations) GarbageCollect(args *structs.AllocSpecificRequest, reply *structs.GenericResponse) error {
	// We only allow stale reads since the only potentially stale information is
	// the Node registration and the cost is fairly high for adding another hop
	// in the forwarding chain.
	args.QueryOptions.AllowStale = true

	// Potentially forward to a different region.
	if done, err := a.srv.forward("ClientAllocations.GarbageCollect", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "client_allocations", "garbage_collect"}, time.Now())

	// Verify the arguments.
	if args.AllocID == "" {
		return errors.New("missing AllocID")
	}

	// Find the allocation
	snap, err := a.srv.State().Snapshot()
	if err != nil {
		return err
	}

	alloc, err := getAlloc(snap, args.AllocID)
	if err != nil {
		return err
	}

	// Check namespace submit-job permission.
	if aclObj, err := a.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowNsOp(alloc.Namespace, acl.NamespaceCapabilitySubmitJob) {
		return structs.ErrPermissionDenied
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

// Restart is used to trigger a restart of an allocation or a subtask on a client.
func (a *ClientAllocations) Restart(args *structs.AllocRestartRequest, reply *structs.GenericResponse) error {
	// We only allow stale reads since the only potentially stale information is
	// the Node registration and the cost is fairly high for adding another hop
	// in the forwarding chain.
	args.QueryOptions.AllowStale = true

	// Potentially forward to a different region.
	if done, err := a.srv.forward("ClientAllocations.Restart", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "client_allocations", "restart"}, time.Now())

	// Find the allocation
	snap, err := a.srv.State().Snapshot()
	if err != nil {
		return err
	}

	alloc, err := getAlloc(snap, args.AllocID)
	if err != nil {
		return err
	}

	// Check for namespace alloc-lifecycle permissions.
	if aclObj, err := a.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowNsOp(alloc.Namespace, acl.NamespaceCapabilityAllocLifecycle) {
		return structs.ErrPermissionDenied
	}

	// Make sure Node is valid and new enough to support RPC
	_, err = getNodeForRpc(snap, alloc.NodeID)
	if err != nil {
		return err
	}

	// Get the connection to the client
	state, ok := a.srv.getNodeConn(alloc.NodeID)
	if !ok {
		return findNodeConnAndForward(a.srv, alloc.NodeID, "ClientAllocations.Restart", args, reply)
	}

	// Make the RPC
	return NodeRpc(state.Session, "Allocations.Restart", args, reply)
}

// Stats is used to collect allocation statistics
func (a *ClientAllocations) Stats(args *cstructs.AllocStatsRequest, reply *cstructs.AllocStatsResponse) error {
	// We only allow stale reads since the only potentially stale information is
	// the Node registration and the cost is fairly high for adding another hop
	// in the forwarding chain.
	args.QueryOptions.AllowStale = true

	// Potentially forward to a different region.
	if done, err := a.srv.forward("ClientAllocations.Stats", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "client_allocations", "stats"}, time.Now())

	// Find the allocation
	snap, err := a.srv.State().Snapshot()
	if err != nil {
		return err
	}

	alloc, err := getAlloc(snap, args.AllocID)
	if err != nil {
		return err
	}

	// Check for namespace read-job permissions.
	if aclObj, err := a.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowNsOp(alloc.Namespace, acl.NamespaceCapabilityReadJob) {
		return structs.ErrPermissionDenied
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

// exec is used to execute command in a running task
func (a *ClientAllocations) exec(conn io.ReadWriteCloser) {
	defer conn.Close()
	defer metrics.MeasureSince([]string{"nomad", "alloc", "exec"}, time.Now())

	// Decode the arguments
	var args cstructs.AllocExecRequest
	decoder := codec.NewDecoder(conn, structs.MsgpackHandle)
	encoder := codec.NewEncoder(conn, structs.MsgpackHandle)

	if err := decoder.Decode(&args); err != nil {
		handleStreamResultError(err, helper.Int64ToPtr(500), encoder)
		return
	}

	// Check if we need to forward to a different region
	if r := args.RequestRegion(); r != a.srv.Region() {
		forwardRegionStreamingRpc(a.srv, conn, encoder, &args, "Allocations.Exec",
			args.AllocID, &args.QueryOptions)
		return
	}

	// Verify the arguments.
	if args.AllocID == "" {
		handleStreamResultError(errors.New("missing AllocID"), helper.Int64ToPtr(400), encoder)
		return
	}

	// Retrieve the allocation
	snap, err := a.srv.State().Snapshot()
	if err != nil {
		handleStreamResultError(err, nil, encoder)
		return
	}

	alloc, err := getAlloc(snap, args.AllocID)
	if structs.IsErrUnknownAllocation(err) {
		handleStreamResultError(err, helper.Int64ToPtr(404), encoder)
		return
	}
	if err != nil {
		handleStreamResultError(err, nil, encoder)
		return
	}

	// Check node read permissions
	if aclObj, err := a.srv.ResolveToken(args.AuthToken); err != nil {
		handleStreamResultError(err, nil, encoder)
		return
	} else if aclObj != nil && !aclObj.AllowNsOp(alloc.Namespace, acl.NamespaceCapabilityAllocExec) {
		// client ultimately checks if AllocNodeExec is required
		handleStreamResultError(structs.ErrPermissionDenied, nil, encoder)
		return
	}

	nodeID := alloc.NodeID

	// Make sure Node is valid and new enough to support RPC
	node, err := snap.NodeByID(nil, nodeID)
	if err != nil {
		handleStreamResultError(err, helper.Int64ToPtr(500), encoder)
		return
	}

	if node == nil {
		err := fmt.Errorf("Unknown node %q", nodeID)
		handleStreamResultError(err, helper.Int64ToPtr(400), encoder)
		return
	}

	if err := nodeSupportsRpc(node); err != nil {
		handleStreamResultError(err, helper.Int64ToPtr(400), encoder)
		return
	}

	// Get the connection to the client either by forwarding to another server
	// or creating a direct stream
	var clientConn net.Conn
	state, ok := a.srv.getNodeConn(nodeID)
	if !ok {
		// Determine the Server that has a connection to the node.
		srv, err := a.srv.serverWithNodeConn(nodeID, a.srv.Region())
		if err != nil {
			var code *int64
			if structs.IsErrNoNodeConn(err) {
				code = helper.Int64ToPtr(404)
			}
			handleStreamResultError(err, code, encoder)
			return
		}

		// Get a connection to the server
		conn, err := a.srv.streamingRpc(srv, "Allocations.Exec")
		if err != nil {
			handleStreamResultError(err, nil, encoder)
			return
		}

		clientConn = conn
	} else {
		stream, err := NodeStreamingRpc(state.Session, "Allocations.Exec")
		if err != nil {
			handleStreamResultError(err, nil, encoder)
			return
		}
		clientConn = stream
	}
	defer clientConn.Close()

	// Send the request.
	outEncoder := codec.NewEncoder(clientConn, structs.MsgpackHandle)
	if err := outEncoder.Encode(args); err != nil {
		handleStreamResultError(err, nil, encoder)
		return
	}

	structs.Bridge(conn, clientConn)
	return
}

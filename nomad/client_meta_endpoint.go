package nomad

import (
	"errors"
	"time"

	metrics "github.com/armon/go-metrics"
	hclog "github.com/hashicorp/go-hclog"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
)

// Meta endpoints are used to access and modify node metadata.
type ClientMeta struct {
	srv    *Server
	logger hclog.Logger
}

// Get is used to retrieve the current metadata for a given Node. It retreives
// the metadata from the Node directly, rather than from a Server.
func (m *ClientMeta) Get(args *structs.NodeSpecificRequest, reply *cstructs.ClientMetadataResponse) error {
	// We only allow stale reads since the only potentially stale information is
	// the Node registration and the cost is fairly high for adding another hope
	// in the forwarding chain.
	args.QueryOptions.AllowStale = true

	// Potentially forward to a different region.
	if done, err := m.srv.forward("ClientMeta.Get", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "client_metadata", "get"}, time.Now())

	if aclObj, err := m.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowNodeRead() {
		return structs.ErrPermissionDenied
	}

	// Verify the arguments.
	if args.NodeID == "" {
		return errors.New("missing NodeID")
	}

	// Make sure Node is valid and new enough to support RPC
	snap, err := m.srv.State().Snapshot()
	if err != nil {
		return err
	}

	_, err = getNodeForRpc(snap, args.NodeID)
	if err != nil {
		return err
	}

	// Get the connection to the client
	state, ok := m.srv.getNodeConn(args.NodeID)
	if !ok {
		return findNodeConnAndForward(m.srv, args.NodeID, "ClientMeta.Get", args, reply)
	}

	// Make the RPC
	return NodeRpc(state.Session, "ClientMetadata.Metadata", args, reply)
}

// Put is used to replace the metadata for a given Node.
func (m *ClientMeta) Put(args *cstructs.ClientMetadataReplaceRequest, reply *cstructs.ClientMetadataUpdateResponse) error {
	// Potentially forward to a different region.
	if done, err := m.srv.forward("ClientMeta.Put", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "client_metadata", "put"}, time.Now())

	if aclObj, err := m.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowNodeWrite() {
		return structs.ErrPermissionDenied
	}

	// Verify the arguments.
	if args.NodeID == "" {
		return errors.New("missing NodeID")
	}

	if args.Metadata == nil {
		return errors.New("missing Metadata")
	}

	// Make sure Node is valid and new enough to support RPC
	snap, err := m.srv.State().Snapshot()
	if err != nil {
		return err
	}

	_, err = getNodeForRpc(snap, args.NodeID)
	if err != nil {
		return err
	}

	// Get the connection to the client
	state, ok := m.srv.getNodeConn(args.NodeID)
	if !ok {
		return findNodeConnAndForward(m.srv, args.NodeID, "ClientMeta.Put", args, reply)
	}

	// Make the RPC
	return NodeRpc(state.Session, "ClientMetadata.ReplaceMetadata", args, reply)
}

// Patch is used to partilly update the metadata for a given Node.
func (m *ClientMeta) Patch(args *cstructs.ClientMetadataUpdateRequest, reply *cstructs.ClientMetadataUpdateResponse) error {
	// Potentially forward to a different region.
	if done, err := m.srv.forward("ClientMeta.Patch", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "client_metadata", "patch"}, time.Now())

	if aclObj, err := m.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowNodeRead() {
		return structs.ErrPermissionDenied
	}

	// Verify the arguments.
	if args.NodeID == "" {
		return errors.New("missing NodeID")
	}

	if args.Updates == nil {
		return errors.New("missing Updates")
	}

	// Make sure Node is valid and new enough to support RPC
	snap, err := m.srv.State().Snapshot()
	if err != nil {
		return err
	}

	_, err = getNodeForRpc(snap, args.NodeID)
	if err != nil {
		return err
	}

	// Get the connection to the client
	state, ok := m.srv.getNodeConn(args.NodeID)
	if !ok {
		return findNodeConnAndForward(m.srv, args.NodeID, "ClientMeta.Patch", args, reply)
	}

	// Make the RPC
	return NodeRpc(state.Session, "ClientMetadata.UpdateMetadata", args, reply)
}

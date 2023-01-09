package nomad

import (
	"time"

	metrics "github.com/armon/go-metrics"
	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/structs"
	nstructs "github.com/hashicorp/nomad/nomad/structs"
)

type NodeMeta struct {
	srv    *Server
	logger log.Logger
}

func newNodeMetaEndpoint(srv *Server) *NodeMeta {
	n := &NodeMeta{
		srv:    srv,
		logger: srv.logger.Named("client_meta"),
	}
	return n
}

func (n *NodeMeta) Set(args *structs.NodeMetaSetRequest, reply *structs.NodeMetaResponse) error {
	const method = "NodeMeta.Set"
	if done, err := n.srv.forward(method, args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "client_meta", "Set"}, time.Now())

	// Check node write permissions
	if aclObj, err := n.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowNodeWrite() {
		return nstructs.ErrPermissionDenied
	}

	return n.srv.forwardClientRPC(method, args.NodeID, args, reply)
}

func (n *NodeMeta) Read(args *structs.NodeSpecificRequest, reply *structs.NodeMetaResponse) error {
	const method = "NodeMeta.Read"
	if done, err := n.srv.forward(method, args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "client_meta", "read"}, time.Now())

	// Check node read permissions
	if aclObj, err := n.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowNodeRead() {
		return nstructs.ErrPermissionDenied
	}

	return n.srv.forwardClientRPC(method, args.NodeID, args, reply)
}

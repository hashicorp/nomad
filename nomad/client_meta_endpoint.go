// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

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
		logger: srv.logger.Named("node_meta"),
	}
	return n
}

func (n *NodeMeta) Apply(args *structs.NodeMetaApplyRequest, reply *structs.NodeMetaResponse) error {
	const method = "NodeMeta.Apply"

	// Prevent infinite loop between leader and
	// follower-with-the-target-node-connection.
	args.QueryOptions.AllowStale = true

	authErr := n.srv.Authenticate(nil, args)
	if done, err := n.srv.forward(method, args, args, reply); done {
		return err
	}
	n.srv.MeasureRPCRate("node_meta", nstructs.RateMetricRead, args)
	if authErr != nil {
		return nstructs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "client_meta", "apply"}, time.Now())

	// Check node write permissions
	if aclObj, err := n.srv.ResolveACL(args); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowNodeWrite() {
		return nstructs.ErrPermissionDenied
	}

	return n.srv.forwardClientRPC(method, args.NodeID, args, reply)
}

func (n *NodeMeta) Read(args *structs.NodeSpecificRequest, reply *structs.NodeMetaResponse) error {
	const method = "NodeMeta.Read"

	// Prevent infinite loop between leader and
	// follower-with-the-target-node-connection.
	args.QueryOptions.AllowStale = true

	authErr := n.srv.Authenticate(nil, args)
	if done, err := n.srv.forward(method, args, args, reply); done {
		return err
	}
	n.srv.MeasureRPCRate("node_meta", nstructs.RateMetricRead, args)
	if authErr != nil {
		return nstructs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "client_meta", "read"}, time.Now())

	// Check node read permissions
	if aclObj, err := n.srv.ResolveACL(args); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowNodeRead() {
		return nstructs.ErrPermissionDenied
	}

	return n.srv.forwardClientRPC(method, args.NodeID, args, reply)
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"time"

	metrics "github.com/hashicorp/go-metrics/compat"
	"github.com/hashicorp/nomad/nomad/structs"
)

type NodeIdentity struct {
	srv *Server
}

func newNodeIdentityEndpoint(srv *Server) *NodeIdentity {
	return &NodeIdentity{
		srv: srv,
	}
}

func (n *NodeIdentity) Renew(args *structs.NodeIdentityRenewReq, reply *structs.NodeIdentityRenewResp) error {

	// Prevent infinite loop between the leader and the follower with the target
	// node connection.
	args.QueryOptions.AllowStale = true

	authErr := n.srv.Authenticate(nil, args)
	if done, err := n.srv.forward(structs.NodeIdentityRenewRPCMethod, args, args, reply); done {
		return err
	}
	n.srv.MeasureRPCRate("client_identity", structs.RateMetricRead, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "client_identity", "renew"}, time.Now())

	// Check node write permissions
	if aclObj, err := n.srv.ResolveACL(args); err != nil {
		return err
	} else if !aclObj.AllowNodeWrite() {
		return structs.ErrPermissionDenied
	}

	return n.srv.forwardClientRPC(structs.NodeIdentityRenewRPCMethod, args.NodeID, args, reply)
}

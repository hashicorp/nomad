// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"github.com/hashicorp/nomad/nomad/structs"
)

type NodeIdentity struct {
	c *Client
}

func newNodeIdentityEndpoint(c *Client) *NodeIdentity {
	n := &NodeIdentity{c: c}
	return n
}

func (n *NodeIdentity) Renew(args *structs.NodeIdentityRenewReq, _ *structs.NodeIdentityRenewResp) error {

	// Check node write permissions.
	if aclObj, err := n.c.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if !aclObj.AllowNodeWrite() {
		return structs.ErrPermissionDenied
	}

	// Store the node identity renewal request on the client, so it can be
	// picked up at the next heartbeat.
	n.c.identityForceRenewal.Store(true)

	return nil
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"fmt"

	"github.com/go-jose/go-jose/v3/jwt"
	"github.com/hashicorp/nomad/nomad/structs"
)

type NodeIdentity struct {
	c *Client
}

func newNodeIdentityEndpoint(c *Client) *NodeIdentity {
	n := &NodeIdentity{c: c}
	return n
}

func (n *NodeIdentity) Get(args *structs.NodeIdentityGetReq, resp *structs.NodeIdentityGetResp) error {

	// Check for node read permissions.
	if aclObj, err := n.c.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if !aclObj.AllowNodeRead() {
		return structs.ErrPermissionDenied
	}

	// Parse the signed JWT token from the node identity and extract the claims
	// into a map. This is done to avoid exposing the key material of the signed
	// JWT token, but still results in all the claims which is perfect for
	// debugging and introspection purposes.
	parsedJWT, err := jwt.ParseSigned(n.c.nodeIdentityToken())
	if err != nil {
		return fmt.Errorf("failed to parsed signed token: %w", err)
	}

	claims := make(map[string]any)

	if err := parsedJWT.UnsafeClaimsWithoutVerification(&claims); err != nil {
		return fmt.Errorf("failed to extract claims from token: %w", err)
	}

	resp.Claims = claims
	return nil
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

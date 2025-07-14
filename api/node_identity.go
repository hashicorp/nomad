// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

type NodeIdentityRenewRequest struct {
	NodeID string
}

type NodeIdentityRenewResponse struct{}

type NodeIdentity struct {
	client *Client
}

func (n *Nodes) Identity() *NodeIdentity {
	return &NodeIdentity{client: n.client}
}

func (n *NodeIdentity) Renew(req *NodeIdentityRenewRequest, qo *QueryOptions) (*NodeIdentityRenewResponse, error) {
	var out NodeIdentityRenewResponse
	_, err := n.client.postQuery("/v1/client/identity/renew", req, &out, qo)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

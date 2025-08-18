// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

// NodeIdentityGetRequest represents the request to retrieve the node identity
// claims for a specific node.
type NodeIdentityGetRequest struct {
	NodeID string
}

// NodeIdentityGetResponse represents the response containing the node identity
// claims.
type NodeIdentityGetResponse struct {
	Claims map[string]any
}

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

// Get retrieves the node identity claims for the node specified within the
// request object.
//
// The request uses query options to control the forwarding behavior of the
// request only. Parameters such as Filter, WaitTime, and WaitIndex are not used
// and ignored.
func (n *NodeIdentity) Get(req *NodeIdentityGetRequest, qo *QueryOptions) (*NodeIdentityGetResponse, error) {

	if qo == nil {
		qo = &QueryOptions{}
	}

	if qo.Params == nil {
		qo.Params = make(map[string]string)
	}

	if req.NodeID != "" {
		qo.Params["node_id"] = req.NodeID
	}

	var out NodeIdentityGetResponse

	if _, err := n.client.query("/v1/client/identity", &out, qo); err != nil {
		return nil, err
	}
	return &out, nil
}

// Renew instructs the node to request a new identity from the server at its
// next heartbeat.
//
// The request uses query options to control the forwarding behavior of the
// request only. Parameters such as Filter, WaitTime, and WaitIndex are not used
// and ignored.
func (n *NodeIdentity) Renew(req *NodeIdentityRenewRequest, qo *QueryOptions) (*NodeIdentityRenewResponse, error) {
	var out NodeIdentityRenewResponse
	_, err := n.client.postQuery("/v1/client/identity/renew", req, &out, qo)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

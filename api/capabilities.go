// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

type Capabilities struct {
	client *Client
}

type CapabilitiesList struct {
	ACL              bool
	ACLEnabled       bool
	OIDC             bool
	WorkloadIdentity bool
	ConsulVaultWI    bool
	NUMA             bool
}

func (c *Client) Capabilities() *Capabilities {
	return &Capabilities{client: c}
}

// List returns a list of all capabilities.
func (c *Capabilities) List(q *QueryOptions) (*CapabilitiesList, *QueryMeta, error) {
	var resp *CapabilitiesList
	qm, err := c.client.query("/v1/capabilities", &resp, q)
	if err != nil {
		return nil, nil, err
	}
	return resp, qm, nil
}

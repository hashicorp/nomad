// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package config

type ServerWorkloadSecurityRingsConfig struct {
	PEMPublicKey *string `hcl:"public_pem_key"`
}

func (d *ServerWorkloadSecurityRingsConfig) Copy() *ServerWorkloadSecurityRingsConfig {
	if d == nil {
		return nil
	}

	nd := new(ServerWorkloadSecurityRingsConfig)
	*nd = *d
	return nd
}

func (d *ServerWorkloadSecurityRingsConfig) Merge(o *ServerWorkloadSecurityRingsConfig) *ServerWorkloadSecurityRingsConfig {
	switch {
	case d == nil:
		return o.Copy()
	case o == nil:
		return d.Copy()
	default:
		nd := d.Copy()
		if len(*o.PEMPublicKey) != 0 {
			nd.PEMPublicKey = o.PEMPublicKey
		}
		return nd
	}
}

type ClientWorkloadSecurityRingsConfig struct {
	NodeType *string `hcl:"node_type"`
}

func (d *ClientWorkloadSecurityRingsConfig) Copy() *ClientWorkloadSecurityRingsConfig {
	if d == nil {
		return nil
	}

	nd := new(ClientWorkloadSecurityRingsConfig)
	*nd = *d
	return nd
}

func (d *ClientWorkloadSecurityRingsConfig) Merge(o *ClientWorkloadSecurityRingsConfig) *ClientWorkloadSecurityRingsConfig {
	switch {
	case d == nil:
		return o.Copy()
	case o == nil:
		return d.Copy()
	default:
		nd := d.Copy()
		if len(*o.NodeType) != 0 {
			nd.NodeType = o.NodeType
		}
		return nd
	}
}

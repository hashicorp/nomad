// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/nomad/structs"
)

type Capabilities struct {
	srv    *Server
	ctx    *RPCContext
	logger hclog.Logger
}

func NewCapabilitiesEndpoint(srv *Server, ctx *RPCContext) *Capabilities {
	return &Capabilities{
		srv:    srv,
		ctx:    ctx,
		logger: srv.logger.Named("capabilities"),
	}
}

func (c *Capabilities) List(args *structs.CapabilitiesListRequest, reply *structs.CapabilitiesListResponse) error {
	if done, err := c.srv.forward("Capabilities.List", args, args, reply); done {
		return err
	}

	capabilities := &structs.Capabilities{
		ACL: true,
	}

	acls, _ := c.srv.auth.ResolveACL(&structs.ACLAuthMethodListRequest{})
	capabilities.ACLEnabled = acls != acl.ACLsDisabledACL || acls != nil

	reply.Capabilities = capabilities

	return nil
}

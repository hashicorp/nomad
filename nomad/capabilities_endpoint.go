// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"reflect"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-hclog"

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
	c.srv.MeasureRPCRate("capabilities", structs.RateMetricList, args)
	defer metrics.MeasureSince([]string{"nomad", "capabilities", "list"}, time.Now())

	capabilities := &structs.Capabilities{
		ACL:              fieldExistsInStruct(c.srv.config, "ACLEnabled"),
		ACLEnabled:       c.srv.config.ACLEnabled,
		OIDC:             fieldExistsInStruct(c.srv.config, "OIDCIssuer"),
		WorkloadIdentity: fieldExistsInStruct(c.srv, "encrypter"),
		ConsulVaultWI:    fieldExistsInStruct(c.srv.config, "ConsulConfigs"),
	}

	// node pool detection
	if capabilities.ACL {
		serverACL, _ := c.srv.auth.AuthenticateServerOnly(c.ctx, args)
		if serverACL != nil {
			capabilities.NodePools = fieldExistsInStruct(serverACL, "nodePools")
		}
	}

	reply.Capabilities = capabilities

	return nil
}

func fieldExistsInStruct(s any, field string) bool {
	metaStruct := reflect.ValueOf(s).Elem()
	return !(metaStruct.FieldByName(field) == (reflect.Value{}))
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/nomad/nomad/structs"
)

// Region is used to query and list the known regions
type Region struct {
	srv    *Server
	ctx    *RPCContext
	logger hclog.Logger
}

func NewRegionEndpoint(srv *Server, ctx *RPCContext) *Region {
	return &Region{srv: srv, ctx: ctx, logger: srv.logger.Named("region")}
}

// List is used to list all of the known regions. No leader forwarding is
// required for this endpoint because memberlist is used to populate the
// peers list we read from.
func (r *Region) List(args *structs.GenericRequest, reply *[]string) error {
	// note: we're intentionally throwing away any auth error here and only
	// authenticate so that we can measure rate metrics
	r.srv.Authenticate(r.ctx, args)
	r.srv.MeasureRPCRate("region", structs.RateMetricList, args)

	*reply = r.srv.Regions()
	return nil
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"time"

	metrics "github.com/armon/go-metrics"
	log "github.com/hashicorp/go-hclog"
	nstructs "github.com/hashicorp/nomad/nomad/structs"

	"github.com/hashicorp/nomad/client/structs"
)

// ClientStats is used to forward RPC requests to the targed Nomad client's
// ClientStats endpoint.
type ClientStats struct {
	srv    *Server
	logger log.Logger
}

func NewClientStatsEndpoint(srv *Server) *ClientStats {
	return &ClientStats{srv: srv, logger: srv.logger.Named("client_stats")}
}

func (s *ClientStats) Stats(args *nstructs.NodeSpecificRequest, reply *structs.ClientStatsResponse) error {

	// We only allow stale reads since the only potentially stale information is
	// the Node registration and the cost is fairly high for adding another hope
	// in the forwarding chain.
	args.QueryOptions.AllowStale = true
	authErr := s.srv.Authenticate(nil, args)

	// Potentially forward to a different region.
	if done, err := s.srv.forward("ClientStats.Stats", args, args, reply); done {
		return err
	}
	s.srv.MeasureRPCRate("client_stats", nstructs.RateMetricRead, args)
	if authErr != nil {
		return nstructs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "client_stats", "stats"}, time.Now())

	// Check node read permissions
	if aclObj, err := s.srv.ResolveACL(args); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowNodeRead() {
		return nstructs.ErrPermissionDenied
	}

	return s.srv.forwardClientRPC("ClientStats.Stats", args.NodeID, args, reply)
}

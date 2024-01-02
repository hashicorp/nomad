// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/nomad/client/structs"
	nstructs "github.com/hashicorp/nomad/nomad/structs"
)

// ClientStats endpoint is used for retrieving stats about a client
type ClientStats struct {
	c *Client
}

// Stats is used to retrieve the Clients stats.
func (s *ClientStats) Stats(args *nstructs.NodeSpecificRequest, reply *structs.ClientStatsResponse) error {
	defer metrics.MeasureSince([]string{"client", "client_stats", "stats"}, time.Now())

	// Check node read permissions
	if aclObj, err := s.c.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowNodeRead() {
		return nstructs.ErrPermissionDenied
	}

	clientStats := s.c.StatsReporter()
	reply.HostStats = clientStats.LatestHostStats()
	return nil
}

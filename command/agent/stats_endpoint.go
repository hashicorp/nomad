// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"net/http"
	"strings"

	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
)

func (s *HTTPServer) ClientStatsRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {

	// Build the request and get the requested Node ID
	args := structs.NodeSpecificRequest{}
	s.parse(resp, req, &args.QueryOptions.Region, &args.QueryOptions)
	parseNode(req, &args.NodeID)

	// Determine the handler to use
	useLocalClient, useClientRPC, useServerRPC := s.rpcHandlerForNode(args.NodeID)

	// Make the RPC
	var reply cstructs.ClientStatsResponse
	var rpcErr error
	if useLocalClient {
		rpcErr = s.agent.Client().ClientRPC("ClientStats.Stats", &args, &reply)
	} else if useClientRPC {
		rpcErr = s.agent.Client().RPC("ClientStats.Stats", &args, &reply)
	} else if useServerRPC {
		rpcErr = s.agent.Server().RPC("ClientStats.Stats", &args, &reply)
	} else {
		rpcErr = CodedError(400, "No local Node and node_id not provided")
	}

	if rpcErr != nil {
		if structs.IsErrNoNodeConn(rpcErr) {
			rpcErr = CodedError(404, rpcErr.Error())
		} else if strings.Contains(rpcErr.Error(), "Unknown node") {
			rpcErr = CodedError(404, rpcErr.Error())
		}

		return nil, rpcErr
	}

	return reply.HostStats, nil
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"net/http"

	"github.com/hashicorp/nomad/nomad/structs"
)

func (s *HTTPServer) NodeIdentityRenewRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Build the request by parsing all common parameters and node id
	args := structs.NodeIdentityRenewReq{}
	s.parse(resp, req, &args.QueryOptions.Region, &args.QueryOptions)
	parseNode(req, &args.NodeID)

	// Determine the handler to use
	useLocalClient, useClientRPC, useServerRPC := s.rpcHandlerForNode(args.NodeID)

	// Make the RPC
	var reply structs.NodeIdentityRenewResp
	var rpcErr error
	if useLocalClient {
		rpcErr = s.agent.Client().ClientRPC(structs.NodeIdentityRenewRPCMethod, &args, &reply)
	} else if useClientRPC {
		rpcErr = s.agent.Client().RPC(structs.NodeIdentityRenewRPCMethod, &args, &reply)
	} else if useServerRPC {
		rpcErr = s.agent.Server().RPC(structs.NodeIdentityRenewRPCMethod, &args, &reply)
	} else {
		rpcErr = CodedError(400, "no local Node and node_id not provided")
	}

	if rpcErr != nil {
		if structs.IsErrNoNodeConn(rpcErr) {
			rpcErr = CodedError(404, rpcErr.Error())
		}

		return nil, rpcErr
	}

	return reply, nil
}

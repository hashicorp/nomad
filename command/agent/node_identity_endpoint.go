// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"net/http"

	"github.com/hashicorp/nomad/nomad/structs"
)

func (s *HTTPServer) NodeIdentityGetRequest(resp http.ResponseWriter, req *http.Request) (any, error) {

	if req.Method != http.MethodGet {
		return nil, CodedError(http.StatusMethodNotAllowed, ErrInvalidMethod)
	}

	// Build the request by parsing all common parameters and node id
	args := structs.NodeIdentityGetReq{}
	s.parse(resp, req, &args.QueryOptions.Region, &args.QueryOptions)
	parseNode(req, &args.NodeID)

	// Determine the handler to use
	useLocalClient, useClientRPC, useServerRPC := s.rpcHandlerForNode(args.NodeID)

	// Make the RPC
	var reply structs.NodeIdentityGetResp
	var rpcErr error
	if useLocalClient {
		rpcErr = s.agent.Client().ClientRPC(structs.NodeIdentityGetRPCMethod, &args, &reply)
	} else if useClientRPC {
		rpcErr = s.agent.Client().RPC(structs.NodeIdentityGetRPCMethod, &args, &reply)
	} else if useServerRPC {
		rpcErr = s.agent.Server().RPC(structs.NodeIdentityGetRPCMethod, &args, &reply)
	} else {
		rpcErr = CodedError(http.StatusBadRequest, "no local Node and node_id not provided")
	}

	if rpcErr != nil {
		if structs.IsErrNoNodeConn(rpcErr) {
			rpcErr = CodedError(http.StatusNotFound, rpcErr.Error())
		}
		return nil, rpcErr
	}

	return reply, nil
}

func (s *HTTPServer) NodeIdentityRenewRequest(resp http.ResponseWriter, req *http.Request) (any, error) {

	// Only allow POST and PUT methods.
	if !(req.Method == http.MethodPut || req.Method == http.MethodPost) {
		return nil, CodedError(http.StatusMethodNotAllowed, ErrInvalidMethod)
	}

	// Build the request by decoding the request body which will contain the
	// node ID and the common parameters.
	args := structs.NodeIdentityRenewReq{}
	s.parse(resp, req, &args.QueryOptions.Region, &args.QueryOptions)
	parseNode(req, &args.NodeID)

	// If the request body is not empty, it is likely the caller is using this
	// to indicate the node ID. Decode it.
	if req.Body != nil && req.Body != http.NoBody {
		if err := decodeBody(req, &args); err != nil {
			return nil, CodedError(http.StatusBadRequest, err.Error())
		}
	}

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
		rpcErr = CodedError(http.StatusBadRequest, "no local Node and node_id not provided")
	}

	if rpcErr != nil {
		if structs.IsErrNoNodeConn(rpcErr) {
			rpcErr = CodedError(http.StatusNotFound, rpcErr.Error())
		}

		return nil, rpcErr
	}

	return reply, nil
}

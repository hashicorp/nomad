// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"net/http"

	"github.com/hashicorp/nomad/nomad/structs"
)

func (s *HTTPServer) NodeMetaRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	switch req.Method {
	case http.MethodGet:
		return s.nodeMetaRead(resp, req)
	case http.MethodPost:
		return s.nodeMetaApply(resp, req)
	default:
		return nil, CodedError(405, ErrInvalidMethod)
	}
}

func (s *HTTPServer) nodeMetaRead(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Build the request by parsing all common parameters and node id
	args := structs.NodeSpecificRequest{}
	s.parse(resp, req, &args.QueryOptions.Region, &args.QueryOptions)
	parseNode(req, &args.NodeID)

	// Determine the handler to use
	useLocalClient, useClientRPC, useServerRPC := s.rpcHandlerForNode(args.NodeID)

	// Make the RPC
	const rpc = "NodeMeta.Read"
	var reply structs.NodeMetaResponse
	var rpcErr error
	if useLocalClient {
		rpcErr = s.agent.Client().ClientRPC(rpc, &args, &reply)
	} else if useClientRPC {
		rpcErr = s.agent.Client().RPC(rpc, &args, &reply)
	} else if useServerRPC {
		rpcErr = s.agent.Server().RPC(rpc, &args, &reply)
	} else {
		rpcErr = CodedError(400, "No local Node and node_id not provided")
	}

	if rpcErr != nil {
		if structs.IsErrNoNodeConn(rpcErr) {
			rpcErr = CodedError(404, rpcErr.Error())
		}

		return nil, rpcErr
	}

	return reply, nil
}

func (s *HTTPServer) nodeMetaApply(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Build the request by decoding body and then parsing all common
	// parameters and node id
	args := structs.NodeMetaApplyRequest{}
	if err := decodeBody(req, &args); err != nil {
		return nil, CodedError(http.StatusBadRequest, err.Error())
	}

	s.parse(resp, req, &args.QueryOptions.Region, &args.QueryOptions)
	parseNode(req, &args.NodeID)

	// Determine the handler to use
	useLocalClient, useClientRPC, useServerRPC := s.rpcHandlerForNode(args.NodeID)

	// Make the RPC
	const method = "NodeMeta.Apply"
	var reply structs.NodeMetaResponse
	var rpcErr error
	if useLocalClient {
		rpcErr = s.agent.Client().ClientRPC(method, &args, &reply)
	} else if useClientRPC {
		rpcErr = s.agent.Client().RPC(method, &args, &reply)
	} else if useServerRPC {
		rpcErr = s.agent.Server().RPC(method, &args, &reply)
	} else {
		rpcErr = CodedError(400, "No local Node and node_id not provided")
	}

	if rpcErr != nil {
		if structs.IsErrNoNodeConn(rpcErr) {
			rpcErr = CodedError(404, rpcErr.Error())
		}

		return nil, rpcErr
	}

	return reply, nil

}

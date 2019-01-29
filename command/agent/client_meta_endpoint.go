package agent

import (
	"fmt"
	"net/http"

	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
)

func (s *HTTPServer) ClientMetaRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Get the requested Node ID
	requestedNode := req.URL.Query().Get("node_id")

	switch req.Method {
	case "GET":
		{
			return s.clientMetaGetCurrent(resp, req, requestedNode)
		}
	case "PUT":
		{
			return s.clientMetaReplace(resp, req, requestedNode)
		}
	case "PATCH":
		{
			return s.clientMetaUpdate(resp, req, requestedNode)
		}
	default:
		return nil, CodedError(405, fmt.Sprintf("Unsupported http method (%s)", req.Method))
	}
}

func (s *HTTPServer) clientMetaGetCurrent(resp http.ResponseWriter, req *http.Request, nodeID string) (interface{}, error) {
	// Build the request and parse the ACL token
	args := structs.NodeSpecificRequest{
		NodeID: nodeID,
	}
	s.parse(resp, req, &args.QueryOptions.Region, &args.QueryOptions)

	// Determine the handler to use
	useLocalClient, useClientRPC, useServerRPC := s.rpcHandlerForNode(nodeID)

	// Make the RPC
	var reply cstructs.ClientMetadataResponse
	var rpcErr error
	if useLocalClient {
		rpcErr = s.agent.Client().ClientRPC("ClientMetadata.Metadata", &args, &reply)
	} else if useClientRPC {
		rpcErr = s.agent.Client().RPC("ClientMeta.Get", &args, &reply)
	} else if useServerRPC {
		rpcErr = s.agent.Server().RPC("ClientMeta.Get", &args, &reply)
	} else {
		rpcErr = CodedError(400, "No local Node and node_id not provided")
	}

	if rpcErr != nil {
		if structs.IsErrNoNodeConn(rpcErr) {
			rpcErr = CodedError(404, rpcErr.Error())
		}
	}

	return &reply, rpcErr
}

func (s *HTTPServer) clientMetaReplace(resp http.ResponseWriter, req *http.Request, nodeID string) (interface{}, error) {
	args := cstructs.ClientMetadataReplaceRequest{
		NodeID: nodeID,
	}
	s.parseWriteRequest(req, &args.WriteRequest)

	err := decodeBody(req, &args)
	if err != nil {
		return nil, CodedError(400, fmt.Sprintf("Failed to decode body: %v", err))
	}

	// Determine the handler to use
	useLocalClient, useClientRPC, useServerRPC := s.rpcHandlerForNode(nodeID)

	// Make the RPC
	var reply cstructs.ClientMetadataUpdateResponse
	var rpcErr error
	if useLocalClient {
		rpcErr = s.agent.Client().ClientRPC("ClientMetadata.ReplaceMetadata", &args, &reply)
	} else if useClientRPC {
		rpcErr = s.agent.Client().RPC("ClientMeta.Put", &args, &reply)
	} else if useServerRPC {
		rpcErr = s.agent.Server().RPC("ClientMeta.Put", &args, &reply)
	} else {
		rpcErr = CodedError(400, "No local Node and node_id not provided")
	}

	if rpcErr != nil {
		if structs.IsErrNoNodeConn(rpcErr) {
			rpcErr = CodedError(404, rpcErr.Error())
		}
	}

	return &reply, rpcErr
}

func (s *HTTPServer) clientMetaUpdate(resp http.ResponseWriter, req *http.Request, nodeID string) (interface{}, error) {
	args := cstructs.ClientMetadataUpdateRequest{
		NodeID: nodeID,
	}
	s.parseWriteRequest(req, &args.WriteRequest)

	err := decodeBody(req, &args)
	if err != nil {
		return nil, CodedError(400, fmt.Sprintf("Failed to decode body: %v", err))
	}

	// Determine the handler to use
	useLocalClient, useClientRPC, useServerRPC := s.rpcHandlerForNode(nodeID)

	// Make the RPC
	var reply cstructs.ClientMetadataUpdateResponse
	var rpcErr error
	if useLocalClient {
		rpcErr = s.agent.Client().ClientRPC("ClientMetadata.UpdateMetadata", &args, &reply)
	} else if useClientRPC {
		rpcErr = s.agent.Client().RPC("ClientMeta.Patch", &args, &reply)
	} else if useServerRPC {
		rpcErr = s.agent.Server().RPC("ClientMeta.Patch", &args, &reply)
	} else {
		rpcErr = CodedError(400, "No local Node and node_id not provided")
	}

	if rpcErr != nil {
		if structs.IsErrNoNodeConn(rpcErr) {
			rpcErr = CodedError(404, rpcErr.Error())
		}
	}

	return &reply, rpcErr
}

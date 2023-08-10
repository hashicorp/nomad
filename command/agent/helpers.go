// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

// rpcHandlerForAlloc is a helper that given an allocation ID returns whether to
// use the local clients RPC, the local clients remote RPC or the server on the
// agent.
func (s *HTTPServer) rpcHandlerForAlloc(allocID string) (localClient, remoteClient, server bool) {
	c := s.agent.Client()
	srv := s.agent.Server()

	// See if the local client can handle the request.
	localAlloc := false
	if c != nil {
		// If there is an error it means that the client doesn't have the
		// allocation so we can't use the local client
		_, err := c.GetAllocState(allocID)
		if err == nil {
			localAlloc = true
		}
	}

	// Only use the client RPC to server if we don't have a server and the local
	// client can't handle the call.
	useClientRPC := c != nil && !localAlloc && srv == nil

	// Use the server as a last case.
	useServerRPC := !localAlloc && !useClientRPC && srv != nil

	return localAlloc, useClientRPC, useServerRPC
}

// rpcHandlerForNode is a helper that given a node ID returns whether to
// use the local clients RPC, the local clients remote RPC or the server on the
// agent. If there is a local node and no node id is given, it is assumed the
// local node is being targed.
func (s *HTTPServer) rpcHandlerForNode(nodeID string) (localClient, remoteClient, server bool) {
	c := s.agent.Client()
	srv := s.agent.Server()

	// See if the local client can handle the request.
	localClient = c != nil && // Must have a client
		(nodeID == "" || // If no node ID is given
			nodeID == c.NodeID()) // Requested node is the local node.

	// Only use the client RPC to server if we don't have a server and the local
	// client can't handle the call.
	useClientRPC := c != nil && !localClient && srv == nil

	// Use the server as a last case.
	useServerRPC := !localClient && !useClientRPC && srv != nil && nodeID != ""

	return localClient, useClientRPC, useServerRPC
}

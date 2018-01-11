package agent

import (
	"net/http"
	"strings"

	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad"
)

func (s *HTTPServer) ClientStatsRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Get the requested Node ID
	requestedNode := req.URL.Query().Get("node_id")

	localClient := s.agent.Client()
	localServer := s.agent.Server()

	// See if the local client can handle the request.
	useLocalClient := localClient != nil && // Must have a client
		(requestedNode == "" || // If no node ID is given
			requestedNode == localClient.NodeID()) // Requested node is the local node.

	// Only use the client RPC to server if we don't have a server and the local
	// client can't handle the call.
	useClientRPC := localClient != nil && !useLocalClient && localServer == nil

	// Use the server as a last case.
	useServerRPC := localServer != nil && requestedNode != ""

	// Build the request and parse the ACL token
	args := cstructs.ClientStatsRequest{
		NodeID: requestedNode,
	}
	s.parse(resp, req, &args.QueryOptions.Region, &args.QueryOptions)

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
		if nomad.IsErrNoNodeConn(rpcErr) {
			rpcErr = CodedError(404, rpcErr.Error())
		} else if strings.Contains(rpcErr.Error(), "Unknown node") {
			rpcErr = CodedError(404, rpcErr.Error())
		}

		return nil, rpcErr
	}

	return reply.HostStats, nil
}

package agent

import (
	"net/http"

	cstructs "github.com/hashicorp/nomad/client/structs"
)

func (s *HTTPServer) ClientStatsRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if s.agent.client == nil {
		return nil, clientNotRunning
	}

	// Parse the ACL token
	var args cstructs.ClientStatsRequest
	s.parseToken(req, &args.AuthToken)

	// Make the RPC
	var reply cstructs.ClientStatsResponse
	if err := s.agent.Client().ClientRPC("ClientStats.Stats", &args, &reply); err != nil {
		return nil, err
	}

	return reply.HostStats, nil
}

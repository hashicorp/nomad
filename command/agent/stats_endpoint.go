package agent

import (
	"net/http"

	"github.com/hashicorp/nomad/nomad/structs"
)

func (s *HTTPServer) ClientStatsRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if s.agent.client == nil {
		return nil, clientNotRunning
	}

	var secret string
	s.parseToken(req, &secret)

	// Check node read permissions
	if aclObj, err := s.agent.Client().ResolveToken(secret); err != nil {
		return nil, err
	} else if aclObj != nil && !aclObj.AllowNodeRead() {
		return nil, structs.ErrPermissionDenied
	}

	clientStats := s.agent.client.StatsReporter()
	return clientStats.LatestHostStats(), nil
}

package agent

import "net/http"

const (
	invalidSinceErrPrefix = "can't read the since query parameter"
)

func (s *HTTPServer) ClientStatsRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if s.agent.client == nil {
		return nil, clientNotRunning
	}

	clientStats := s.agent.client.StatsReporter()
	return clientStats.LatestHostStats(), nil
}

package agent

import (
	"fmt"
	"net/http"
	"strconv"
)

func (s *HTTPServer) ClientStatsRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if s.agent.client == nil {
		return nil, clientNotRunning
	}

	var since int
	var err error
	ts := false
	if sinceTime := req.URL.Query().Get("since"); sinceTime != "" {
		ts = true
		since, err = strconv.Atoi(sinceTime)
		if err != nil {
			return nil, CodedError(400, fmt.Sprintf("can't read the since query parameter: %v", err))
		}
	}

	clientStats := s.agent.client.StatsReporter()
	if ts {
		return clientStats.HostStatsTS(int64(since)), nil
	}
	return clientStats.HostStats(), nil
}

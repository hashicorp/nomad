package agent

import (
	"fmt"
	"net/http"
)

func (s *HTTPServer) StatsRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if s.agent.client == nil {
		return nil, clientNotRunning
	}
	var allocID, task string
	if allocID = req.URL.Query().Get("allocation"); allocID == "" {
		return nil, fmt.Errorf("provide a valid alloc id")
	}
	if task = req.URL.Query().Get("task"); task != "" {
		return s.agent.client.ResourceUsageOfTask(allocID, task)
	}
	return s.agent.client.ResourceUsageOfAlloc(allocID)
}

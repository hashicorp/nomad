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
	statsReporter, err := s.agent.client.AllocStats(allocID)
	if err != nil {
		return nil, err
	}
	if task = req.URL.Query().Get("task"); task != "" {
		taskStatsReporter, err := statsReporter.TaskStats(task)
		if err != nil {
			return nil, err
		}
		return taskStatsReporter.ResourceUsage(), nil
	}
	res := make(map[string]interface{})
	for task, sr := range statsReporter.AllocStats() {
		res[task] = sr.ResourceUsage()
	}
	return res, nil
}

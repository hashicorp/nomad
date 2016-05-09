package agent

import (
	"fmt"
	"net/http"
)

func (s *HTTPServer) StatsRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if s.agent.client == nil {
		return nil, clientNotRunning
	}
	cStatsReporter := s.agent.client.StatsReporter()
	var allocID, task string
	if allocID = req.URL.Query().Get("allocation"); allocID == "" {
		return cStatsReporter.HostStats(), nil
	}
	allocStats := cStatsReporter.AllocStats()
	arStatsReporter, ok := allocStats[allocID]
	if !ok {
		return nil, fmt.Errorf("alloc %q is not running on this client", allocID)
	}
	if task = req.URL.Query().Get("task"); task != "" {
		taskStatsReporter, err := arStatsReporter.TaskStats(task)
		if err != nil {
			return nil, err
		}
		return taskStatsReporter.ResourceUsage(), nil
	}
	res := make(map[string]interface{})
	for task, sr := range arStatsReporter.AllocStats() {
		res[task] = sr.ResourceUsage()
	}
	return res, nil
}

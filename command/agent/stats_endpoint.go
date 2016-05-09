package agent

import (
	"fmt"
	"net/http"
)

func (s *HTTPServer) StatsRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if s.agent.client == nil {
		return nil, clientNotRunning
	}

	clientStats := s.agent.client.StatsReporter()

	// Return the host stats if alloc ID is not present
	var allocID, task string
	if allocID = req.URL.Query().Get("allocation"); allocID == "" {
		return clientStats.HostStats(), nil
	}

	// Check if the allocation is running on the node
	allocStats, ok := clientStats.AllocStats()[allocID]
	if !ok {
		return nil, fmt.Errorf("alloc %q is not running on this client", allocID)
	}

	// Return the resource usage of the task if the task name is specified
	if task = req.URL.Query().Get("task"); task != "" {
		taskStats, err := allocStats.TaskStats(task)
		if err != nil {
			return nil, err
		}
		return taskStats.ResourceUsage(), nil
	}

	// Return the resource usage of all the tasks in an allocation if task name
	// is not specified
	res := make(map[string]interface{})
	for task, taskStats := range allocStats.AllocStats() {
		res[task] = taskStats.ResourceUsage()
	}
	return res, nil
}

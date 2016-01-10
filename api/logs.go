package api

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type TaskLogs struct {
	client *Client
}

func (c *Client) TaskLogs() *TaskLogs {
	return &TaskLogs{client: c}
}

func (l *TaskLogs) Get(alloc string, task string, stdout bool, stderr bool, follow bool, lines int64) (io.Reader, error) {
	allocation, _, err := l.client.Allocations().Info(alloc, &QueryOptions{})
	if err != nil {
		return nil, err
	}
	nodeID := allocation.NodeID

	node, _, err := l.client.Nodes().Info(nodeID, &QueryOptions{})
	if err != nil {
		return nil, err
	}
	if node.LogDaemonAddr == "" {
		return nil, fmt.Errorf("log daemon not running on node: %v", nodeID)
	}

	u, _ := url.Parse(fmt.Sprintf("http://%s/v1/logs/%s/%s", node.LogDaemonAddr, alloc, task))

	req := &http.Request{
		Method: "GET",
		URL:    u,
	}
	c := http.Client{}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}

	return resp.Body, nil
}

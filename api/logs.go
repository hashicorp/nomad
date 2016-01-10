package api

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

type TaskLogs struct {
	client *Client
}

func (c *Client) TaskLogs() *TaskLogs {
	return &TaskLogs{client: c}
}

func (l *TaskLogs) Get(alloc string, task string, stdout bool, stderr bool, follow bool, lines int) (io.Reader, error) {
	allocation, _, err := l.client.Allocations().Info(alloc, &QueryOptions{})
	if err != nil {
		return nil, err
	}

	if allocation.ClientStatus == "pending" {
		return nil, fmt.Errorf("task %q hasn't started on the allocation %q", task, alloc)
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
	u.Query().Set("follow", strconv.FormatBool(follow))
	u.Query().Set("lines", strconv.Itoa(lines))
	u.Query().Set("stdout", strconv.FormatBool(stdout))
	u.Query().Set("stderr", strconv.FormatBool(stderr))

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

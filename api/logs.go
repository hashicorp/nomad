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

func (l *TaskLogs) Get(allocation *Allocation, task string, stdout bool, stderr bool, follow bool, lines int) (io.Reader, error) {
	node, _, err := l.client.Nodes().Info(allocation.NodeID, &QueryOptions{})
	if err != nil {
		return nil, err
	}
	if node.LogDaemonAddr == "" {
		return nil, fmt.Errorf("log daemon not running on node: %v", allocation.NodeID)
	}
	u := l.getPath(node.LogDaemonAddr, allocation.ID, task, stdout, stderr, follow, lines)
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

func (l *TaskLogs) getPath(addr string, alloc string, task string,
	stdout bool, stderr bool, follow bool, lines int) *url.URL {
	p := "/v1/logs/%s/%s"

	if !stderr && stdout {
		p = "/v1/logs/%s/%s/stdout"
	}

	if !stdout && stderr {
		p = "/v1/logs/%s/%s/stderr"
	}

	path := fmt.Sprintf(p, alloc, task)

	u := &url.URL{
		Scheme: "http",
		Host:   addr,
		Path:   path,
	}

	v := url.Values{}
	v.Set("follow", strconv.FormatBool(follow))
	v.Set("lines", strconv.Itoa(lines))
	u.RawQuery = v.Encode()

	return u
}

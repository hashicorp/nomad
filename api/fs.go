package api

import (
	"encoding/json"
	"fmt"
	"github.com/hashicorp/nomad/client/allocdir"
	"net/http"
	"net/url"
)

// AllocFS is used to introspect an allocation directory on a Nomad client
type AllocFS struct {
	client *Client
}

// AllocFS returns an handle to the AllocFS endpoints
func (c *Client) AllocFS() *AllocFS {
	return &AllocFS{client: c}
}

// List is used to list the files at a given path of an allocation directory
func (a *AllocFS) List(alloc *Allocation, path string, q *QueryOptions) ([]*allocdir.AllocFileInfo, *QueryMeta, error) {
	node, _, err := a.client.Nodes().Info(alloc.NodeID, &QueryOptions{})
	if err != nil {
		return nil, nil, err
	}

	if node.HTTPAddr == "" {
		return nil, nil, fmt.Errorf("http addr of the node where alloc %q is running is not advertised", alloc.ID)
	}
	u := &url.URL{
		Scheme: "http",
		Host:   node.HTTPAddr,
		Path:   fmt.Sprintf("/v1/client/fs/ls/%s", alloc.ID),
	}
	v := url.Values{}
	v.Set("path", path)
	u.RawQuery = v.Encode()
	req := &http.Request{
		Method: "GET",
		URL:    u,
	}
	c := http.Client{}
	resp, err := c.Do(req)
	if err != nil {
		return nil, nil, err
	}
	decoder := json.NewDecoder(resp.Body)
	var files []*allocdir.AllocFileInfo
	if err := decoder.Decode(&files); err != nil {
		return nil, nil, err
	}
	return files, nil, nil
}

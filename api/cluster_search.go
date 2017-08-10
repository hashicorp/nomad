package api

import (
	"github.com/hashicorp/nomad/nomad/structs"
)

type ClusterSearch struct {
	client *Client
}

// ClusterSearch returns a handle on the ClusterSearch endpoints
func (c *Client) ClusterSearch() *ClusterSearch {
	return &ClusterSearch{client: c}
}

// List returns a list of matches for a particular context and prefix. If a
// context is not specified, matches for all contexts are returned.
func (cs *ClusterSearch) List(prefix, context string) (*structs.ClusterSearchResponse, error) {
	var resp structs.ClusterSearchResponse
	req := &structs.ClusterSearchRequest{Prefix: prefix, Context: context}

	_, err := cs.client.write("/v1/cluster/search", req, &resp, nil)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

package api

import (
	c "github.com/hashicorp/nomad/api/contexts"
	"github.com/hashicorp/nomad/nomad/structs"
)

type Search struct {
	client *Client
}

// Search returns a handle on the Search endpoints
func (c *Client) Search() *Search {
	return &Search{client: c}
}

// PrefixSearch returns a list of matches for a particular context and prefix. If a
// context is not specified, matches for all contexts are returned.
func (s *Search) PrefixSearch(prefix string, context c.Context) (*structs.SearchResponse, error) {
	var resp structs.SearchResponse
	req := &structs.SearchRequest{Prefix: prefix, Context: string(context)}

	_, err := s.client.write("/v1/search", req, &resp, nil)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

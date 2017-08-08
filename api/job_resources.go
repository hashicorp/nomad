package api

import (
	"github.com/hashicorp/nomad/nomad/structs"
)

type JobResources struct {
	client *Client
}

// JobResources returns a handle on the JobResources endpoints
func (c *Client) JobResources() *JobResources {
	return &JobResources{client: c}
}

// List returns a list of all resources for a particular context. If a
// context is not specified, matches for all contezts are returned.
func (j *JobResources) List(prefix string, context string) (*structs.ResourceListResponse, error) {
	var resp structs.ResourceListResponse
	req := &structs.ResourceListRequest{Prefix: prefix, Context: context}

	_, err := j.client.write("/v1/resources/", req, &resp, nil)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

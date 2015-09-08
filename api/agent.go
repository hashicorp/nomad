package api

import (
	"fmt"
)

// Agent encapsulates an API client which talks to Nomad's
// agent endpoints for a specific node.
type Agent struct {
	client *Client
	node   string
}

// Agent returns a new agent which can be used to query
// the agent-specific endpoints.
func (c *Client) Agent() *Agent {
	return &Agent{client: c}
}

// Self is used to query the /v1/agent/self endpoint and
// returns information specific to the running agent.
func (a *Agent) Self() (map[string]map[string]interface{}, error) {
	var out map[string]map[string]interface{}

	// Query the self endpoint on the agent
	_, err := a.client.query("/v1/agent/self", &out, nil)
	if err != nil {
		return nil, fmt.Errorf("failed querying self endpoint: %s", err)
	}

	return out, nil
}

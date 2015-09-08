package api

import (
	"fmt"
)

// Agent encapsulates an API client which talks to Nomad's
// agent endpoints for a specific node.
type Agent struct {
	client *Client

	// Cache static agent info
	nodeName   string
	datacenter string
	region     string
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

	// Populate the cache for faster queries
	a.populateCache(out)

	return out, nil
}

// populateCache is used to insert various pieces of static
// data into the agent handle. This is used during subsequent
// lookups for the same data later on to save the round trip.
func (a *Agent) populateCache(info map[string]map[string]interface{}) {
	if a.nodeName == "" {
		a.nodeName, _ = info["member"]["Name"].(string)
	}
	if tags, ok := info["member"]["Tags"].(map[string]interface{}); ok {
		if a.datacenter == "" {
			a.datacenter, _ = tags["dc"].(string)
		}
		if a.region == "" {
			a.region, _ = tags["region"].(string)
		}
	}
}

// NodeName is used to query the Nomad agent for its node name.
func (a *Agent) NodeName() (string, error) {
	// Return from cache if we have it
	if a.nodeName != "" {
		return a.nodeName, nil
	}

	// Query the node name
	_, err := a.Self()
	return a.nodeName, err
}

// Datacenter is used to return the name of the datacenter which
// the agent is a member of.
func (a *Agent) Datacenter() (string, error) {
	// Return from cache if we have it
	if a.datacenter != "" {
		return a.datacenter, nil
	}

	// Query the agent for the DC
	_, err := a.Self()
	return a.datacenter, err
}

// Region is used to look up the region the agent is in.
func (a *Agent) Region() (string, error) {
	// Return from cache if we have it
	if a.region != "" {
		return a.region, nil
	}

	// Query the agent for the region
	_, err := a.Self()
	return a.region, err
}

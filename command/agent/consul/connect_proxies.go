// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"errors"
)

// ConnectProxies implements SupportedProxiesAPI by using the Consul Agent API.
type ConnectProxies struct {
	agentAPI AgentAPI
}

func NewConnectProxiesClient(agentAPI AgentAPI) *ConnectProxies {
	return &ConnectProxies{
		agentAPI: agentAPI,
	}
}

// Proxies returns a map of the supported proxies. The proxies are sorted from
// Consul with the most preferred version as the 0th element.
//
// If Consul is of a version that does not support the API, a nil map is returned
// with no error.
//
// If Consul cannot be reached an error is returned.
func (c *ConnectProxies) Proxies() (map[string][]string, error) {
	// Based on the Consul query:
	// $ curl -s localhost:8500/v1/agent/self | jq .xDS
	// {
	//  "SupportedProxies": {
	//    "envoy": [
	//      "1.15.0",
	//      "1.14.4",
	//      "1.13.4",
	//      "1.12.6"
	//    ]
	//  }
	// }

	self, err := c.agentAPI.Self()
	if err != nil {
		// this should not fail as long as we can reach consul
		return nil, err
	}

	// If consul does not return a map of the supported consul proxies, it
	// must be a version from before when the API was added in versions
	// 1.9.0, 1.8.3, 1.7.7. Earlier versions in the same point release as well
	// as all of 1.6.X support Connect, but not the supported proxies API.
	// For these cases, we can simply fallback to the old version of Envoy
	// that Nomad defaulted to back then - but not in this logic. Instead,
	// return nil so we can choose what to do at the caller.

	xds, xdsExists := self["xDS"]
	if !xdsExists {
		return nil, nil
	}

	proxies, proxiesExists := xds["SupportedProxies"]
	if !proxiesExists {
		return nil, nil
	}

	// convert interface{} to map[string]interface{}

	intermediate, ok := proxies.(map[string]interface{})
	if !ok {
		return nil, errors.New("unexpected SupportedProxies response format from Consul")
	}

	// convert map[string]interface{} to map[string][]string

	result := make(map[string][]string, len(intermediate))
	for k, v := range intermediate {

		// convert interface{} to []interface{}

		if si, ok := v.([]interface{}); ok {
			ss := make([]string, 0, len(si))
			for _, z := range si {

				// convert interface{} to string

				if s, ok := z.(string); ok {
					ss = append(ss, s)
				}
			}
			result[k] = ss
		}
	}

	return result, nil
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/shoenig/test/must"
)

func TestPlugins_WhenNotClientSkip(t *testing.T) {
	s, _, _ := testServer(t, false, nil)
	must.Nil(t, s.Agent.pluginSingletonLoader)
}

func TestPlugins_WhenClientRun(t *testing.T) {
	s, _, _ := testServer(t, true, nil)
	must.NotNil(t, s.Agent.pluginSingletonLoader)
}

func testServer(t *testing.T, runClient bool, cb func(*Config)) (*TestAgent, *api.Client, string) {
	// Make a new test server
	a := NewTestAgent(t, t.Name(), func(config *Config) {
		config.Client.Enabled = runClient

		if cb != nil {
			cb(config)
		}
	})
	t.Cleanup(a.Shutdown)

	c := a.Client()
	return a, c, a.HTTPAddr()
}

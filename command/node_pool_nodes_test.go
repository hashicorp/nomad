// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"
)

func TestNodePoolNodesCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &NodePoolNodesCommand{}
}

func TestNodePoolNodesCommand_Run(t *testing.T) {
	ci.Parallel(t)

	// Start test server.
	srv, client, url := testServer(t, true, func(c *agent.Config) {
		c.Client.Enabled = false
	})
	testutil.WaitForLeader(t, srv.Agent.RPC)

	// Start some test clients.
	rpcAddr := srv.GetConfig().AdvertiseAddrs.RPC
	clientNodePoolConfig := func(pool string) func(*agent.Config) {
		return func(c *agent.Config) {
			c.Client.NodePool = pool
			c.Client.Servers = []string{rpcAddr}
			c.Client.Enabled = true
			c.Server.Enabled = false
		}
	}

	testClient(t, "client-default", clientNodePoolConfig(""))
	testClient(t, "client-dev", clientNodePoolConfig("dev"))
	testClient(t, "client-prod-1", clientNodePoolConfig("prod"))
	testClient(t, "client-prod-2", clientNodePoolConfig("prod"))
	waitForNodes(t, client)

	nodes, _, err := client.Nodes().List(nil)
	must.NoError(t, err)

	// Nodes().List() sort results by CreateIndex, but for pagination we need
	// nodes sorted by ID.
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].ID < nodes[j].ID
	})

	testCases := []struct {
		name          string
		args          []string
		expectedCode  int
		expectedNodes []string
		expectedErr   string
	}{
		{
			name:         "nodes in prod",
			args:         []string{"prod"},
			expectedCode: 0,
			expectedNodes: []string{
				"client-prod-1",
				"client-prod-2",
			},
		},
		{
			name:         "nodes in all",
			args:         []string{"all"},
			expectedCode: 0,
			expectedNodes: []string{
				"client-default",
				"client-dev",
				"client-prod-1",
				"client-prod-2",
			},
		},
		{
			name:         "filter nodes",
			args:         []string{"-filter", `Name matches "dev"`, "all"},
			expectedCode: 0,
			expectedNodes: []string{
				"client-dev",
			},
		},
		{
			name: "pool by prefix",
			args: []string{"def"},
			expectedNodes: []string{
				"client-default",
			},
		},
		{
			name:         "paginate page 1",
			args:         []string{"-per-page=2", "all"},
			expectedCode: 0,
			expectedNodes: []string{
				nodes[0].Name,
				nodes[1].Name,
			},
		},
		{
			name:         "paginate page 2",
			args:         []string{"-per-page", "2", "-page-token", nodes[2].ID, "all"},
			expectedCode: 0,
			expectedNodes: []string{
				nodes[2].Name,
				nodes[3].Name,
			},
		},
		{
			name:         "missing pool name",
			args:         []string{},
			expectedCode: 1,
			expectedErr:  "This command takes one argument",
		},
		{
			name:         "prefix match multiple",
			args:         []string{"de"},
			expectedCode: 1,
			expectedErr:  "Prefix matched multiple node pools",
		},
		{
			name:         "json and template not allowed",
			args:         []string{"-t", "{{.}}", "all"},
			expectedCode: 1,
			expectedErr:  "Both json and template formatting are not allowed",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Initialize UI and command.
			ui := cli.NewMockUi()
			cmd := &NodePoolNodesCommand{Meta: Meta{Ui: ui}}

			// Run command.
			// Add -json to help parse and validate results.
			args := []string{"-address", url, "-json"}
			args = append(args, tc.args...)
			code := cmd.Run(args)

			if tc.expectedErr != "" {
				must.StrContains(t, ui.ErrorWriter.String(), strings.TrimSpace(tc.expectedErr))
			} else {
				must.Eq(t, "", ui.ErrorWriter.String())

				var nodes []*api.NodeListStub
				err := json.Unmarshal(ui.OutputWriter.Bytes(), &nodes)
				must.NoError(t, err)

				gotNodes := helper.ConvertSlice(nodes,
					func(n *api.NodeListStub) string { return n.Name })
				must.SliceContainsAll(t, tc.expectedNodes, gotNodes)
			}
			must.Eq(t, tc.expectedCode, code)
		})
	}

	t.Run("template formatting", func(t *testing.T) {
		// Initialize UI and command.
		ui := cli.NewMockUi()
		cmd := &NodePoolNodesCommand{Meta: Meta{Ui: ui}}

		// Run command.
		args := []string{"-address", url, "-t", `{{range .}}{{.ID}} {{end}}`, "all"}
		code := cmd.Run(args)
		must.Zero(t, code)

		var expected string
		for _, n := range nodes {
			expected += n.ID + " "
		}
		got := ui.OutputWriter.String()

		must.Eq(t, strings.TrimSpace(expected), strings.TrimSpace(got))
	})
}

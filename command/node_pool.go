// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/go-set/v3"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/api/contexts"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
)

type NodePoolCommand struct {
	Meta
}

func (c *NodePoolCommand) Name() string {
	return "node pool"
}

func (c *NodePoolCommand) Synopsis() string {
	return "Interact with node pools"
}

func (c *NodePoolCommand) Help() string {
	helpText := `
Usage: nomad node pool <subcommand> [options] [args]

  This command groups subcommands for interacting with node pools. Node pools
  are used to partition and control access to a group of nodes. This command
  can be used to create, update, list, and delete node pools.

  Create or update a node pool:

    $ nomad node pool apply <path>

  List all node pools:

    $ nomad node pool list

  Fetch information on an existing node pool:

    $ nomad node info <name>

  Delete a node pool:

    $ nomad node pool delete <name>

  Please refer to individual subcommand help for detailed usage information.
`
	return strings.TrimSpace(helpText)
}

func (c *NodePoolCommand) Run(args []string) int {
	return cli.RunResultHelp
}

func formatNodePoolList(pools []*api.NodePool) string {
	out := make([]string, len(pools)+1)
	out[0] = "Name|Description"
	for i, p := range pools {
		out[i+1] = fmt.Sprintf("%s|%s",
			p.Name,
			p.Description,
		)
	}
	return formatList(out)
}

func nodePoolPredictor(factory ApiClientFactory, filter *set.Set[string]) complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) []string {
		client, err := factory()
		if err != nil {
			return nil
		}

		resp, _, err := client.Search().PrefixSearch(a.Last, contexts.NodePools, nil)
		if err != nil {
			return nil
		}

		results := resp.Matches[contexts.NodePools]
		if filter == nil {
			return results
		}

		filtered := []string{}
		for _, pool := range resp.Matches[contexts.NodePools] {
			if filter.Contains(pool) {
				continue
			}
			filtered = append(filtered, pool)
		}

		return filtered
	})
}

// nodePoolByPrefix returns a node pool that matches the given prefix or a list
// of all matches if an exact match is not found.
func nodePoolByPrefix(client *api.Client, prefix string) (*api.NodePool, []*api.NodePool, error) {
	pools, _, err := client.NodePools().PrefixList(prefix, nil)
	if err != nil {
		return nil, nil, err
	}

	switch len(pools) {
	case 0:
		return nil, nil, fmt.Errorf("No node pool with prefix %q found", prefix)
	case 1:
		return pools[0], nil, nil
	default:
		for _, pool := range pools {
			if pool.Name == prefix {
				return pool, nil, nil
			}
		}
		return nil, pools, nil
	}
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

type WSREnableCommand struct {
	Meta
}

func (c *WSREnableCommand) Name() string {
	return "wsr enable"
}

func (c *WSREnableCommand) Synopsis() string {
	return "Enables workload security rings on the cluster"
}

func (c *WSREnableCommand) Help() string {
	helpText := `
Usage: nomad wsr enable [options] <input>

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Apply Options:

  -public-key
    Public key to use when verifying the content of sensitive workloads.
`
	return strings.TrimSpace(helpText)
}

func (c *WSREnableCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-public-key": complete.PredictNothing,
		})
}

func (c *WSREnableCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *WSREnableCommand) Run(args []string) int {
	var key string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.StringVar(&key, "public-key", "", "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Create node pools
	poolSpec := nodePoolSpec{
		NodePool: &api.NodePool{
			Name:        "trusted_node_pool",
			Description: "Secure Node Pool, automatically created when enabling WSR.",
		},
	}

	// Make API request.
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	_, err = client.NodePools().Register(poolSpec.NodePool, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error applying node pool: %s", err))
		return 1
	}

	c.Ui.Output(fmt.Sprintf("Successfully applied node pool %q!", poolSpec.NodePool.Name))
	return 0
}

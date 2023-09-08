// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/hashicorp/hcl/v2/hclsimple"
	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

type NodePoolApplyCommand struct {
	Meta
}

func (c *NodePoolApplyCommand) Name() string {
	return "node pool apply"
}

func (c *NodePoolApplyCommand) Synopsis() string {
	return "Create or update a node pool"
}

func (c *NodePoolApplyCommand) Help() string {
	helpText := `
Usage: nomad node pool apply [options] <input>

  Apply is used to create or update a node pool. The specification file is read
  from stdin by specifying "-", otherwise a path to the file is expected.

  If ACLs are enabled, this command requires a token with the 'write' capability
  in a 'node_pool' policy that matches the node pool being targeted. In
  federated clusters, the node pool will be created in the authoritative region
  and will be replicated to all federated regions.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Apply Options:

  -json
    Parse the input as a JSON node pool specification.
`
	return strings.TrimSpace(helpText)
}

func (c *NodePoolApplyCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-json": complete.PredictNothing,
		})
}

func (c *NodePoolApplyCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictOr(
		complete.PredictFiles("*.hcl"),
		complete.PredictFiles("*.json"),
	)
}

func (c *NodePoolApplyCommand) Run(args []string) int {
	var jsonInput bool

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&jsonInput, "json", false, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we only have one argument.
	args = flags.Args()
	if len(args) != 1 {
		c.Ui.Error("This command takes one argument: <input>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	// Read input content.
	path := args[0]
	var content []byte
	var err error
	switch path {
	case "-":
		content, err = io.ReadAll(os.Stdin)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Failed to read stdin: %v", err))
			return 1
		}
		// Set .hcl extension so the decoder doesn't fail.
		if !jsonInput {
			path = "stdin.nomad.hcl"
		}
	default:
		content, err = os.ReadFile(path)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Failed to read file %q: %v", path, err))
			return 1
		}
	}

	// Parse input.
	var poolSpec nodePoolSpec
	if jsonInput {
		err = json.Unmarshal(content, &poolSpec.NodePool)
	} else {
		err = hclsimple.Decode(path, content, nil, &poolSpec)
	}
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to parse input content: %v", err))
		return 1
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

type nodePoolSpec struct {
	NodePool *api.NodePool `hcl:"node_pool,block"`
}

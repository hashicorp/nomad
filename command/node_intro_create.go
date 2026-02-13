// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

type NodeIntroCreateCommand struct {
	Meta

	// Fields for the command flags.
	json     bool
	tmpl     string
	ttl      string
	nodeName string
	nodePool string
}

func (n *NodeIntroCreateCommand) Help() string {
	helpText := `
Usage: nomad node intro create [options]

  Generates a new node introduction token. This token is used to authenticate
  a new Nomad client node to the cluster.

  If ACLs are enabled, this command requires a token with the 'node:write'
  capability.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

Create Options:

  -node-name
    The name of the node to which the introduction token will be scoped. If not
    specified, the value will be left empty.

  -node-pool
    The node pool to which the introduction token will be scoped. If not
    specified, the value "default" will be used.

  -ttl
    The TTL to apply to the introduction token. If not specified, the server
    configured default value will be used.

  -json
    Output the response object in JSON format.

  -t
    Format and display the response object using a Go template.
`

	return strings.TrimSpace(helpText)
}

func (n *NodeIntroCreateCommand) Synopsis() string {
	return "Generate a new node introduction token"
}

func (n *NodeIntroCreateCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(n.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-node-pool": nodePoolPredictor(n.Client, nil),
			"-json":      complete.PredictNothing,
			"-t":         complete.PredictAnything,
			"-node-name": complete.PredictAnything,
			"-ttl":       complete.PredictAnything,
		})
}

func (n *NodeIntroCreateCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (n *NodeIntroCreateCommand) Name() string { return "node intro create" }

func (n *NodeIntroCreateCommand) Run(args []string) int {

	flags := n.Meta.FlagSet(n.Name(), FlagSetClient)
	flags.Usage = func() { n.Ui.Output(n.Help()) }
	flags.StringVar(&n.ttl, "ttl", "", "")
	flags.StringVar(&n.nodeName, "node-name", "", "")
	flags.StringVar(&n.nodePool, "node-pool", "", "")
	flags.StringVar(&n.tmpl, "t", "", "")
	flags.BoolVar(&n.json, "json", false, "")
	if err := flags.Parse(args); err != nil {
		return 1
	}

	args = flags.Args()
	if len(args) != 0 {
		n.Ui.Error(uiMessageNoArguments)
		n.Ui.Error(commandErrorText(n))
		return 1
	}

	var ttlTime time.Duration

	if n.ttl != "" {
		parsedTTL, err := time.ParseDuration(n.ttl)
		if err != nil {
			n.Ui.Error(fmt.Sprintf("Error parsing TTL: %s", err))
			return 1
		}
		ttlTime = parsedTTL
	}

	client, err := n.Meta.Client()
	if err != nil {
		n.Ui.Error(fmt.Sprintf("Error creating Nomad client: %s", err))
		return 1
	}

	req := api.ACLIdentityClientIntroductionTokenRequest{
		TTL:      ttlTime,
		NodeName: n.nodeName,
		NodePool: n.nodePool,
	}

	resp, _, err := client.ACLIdentity().CreateClientIntroductionToken(&req, nil)
	if err != nil {
		n.Ui.Error(fmt.Sprintf("Error generating introduction token: %s", err))
		return 1
	}

	if n.json || n.tmpl != "" {
		out, err := Format(n.json, n.tmpl, resp)
		if err != nil {
			n.Ui.Error(err.Error())
			return 1
		}

		n.Ui.Output(out)
		return 0
	}

	n.Ui.Output(fmt.Sprintf("Successfully generated client introduction token:\n\n%s", resp.JWT))
	return 0
}

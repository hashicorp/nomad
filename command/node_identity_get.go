// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

type NodeIdentityGetCommand struct {
	Meta

	// Command flags are stored below for use across the command.
	json bool
	tmpl string
}

func (n *NodeIdentityGetCommand) Help() string {
	helpText := `
Usage: nomad node identity get [options] <node_id>

  Get the identity claims for a node. This command only applies to client
  agents.

  If ACLs are enabled, this command requires a token with the 'node:read'
  capability.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

Get Options:

  -json
    Output the node identity claims in a JSON format.

  -t
    Format and display the node identity claims using a Go template.
`

	return strings.TrimSpace(helpText)
}

func (n *NodeIdentityGetCommand) Synopsis() string { return "Detail a node's identity claims" }

func (n *NodeIdentityGetCommand) Name() string { return "node identity get" }

func (n *NodeIdentityGetCommand) Run(args []string) int {

	flags := n.Meta.FlagSet(n.Name(), FlagSetClient)
	flags.BoolVar(&n.json, "json", false, "")
	flags.StringVar(&n.tmpl, "t", "", "")
	flags.Usage = func() { n.Ui.Output(n.Help()) }

	if err := flags.Parse(args); err != nil {
		return 1
	}
	args = flags.Args()

	if len(args) != 1 {
		n.Ui.Error("This command takes one argument: <node_id>")
		n.Ui.Error(commandErrorText(n))
		return 1
	}

	// Get the HTTP client
	client, err := n.Meta.Client()
	if err != nil {
		n.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	nodeID, err := lookupNodeID(client.Nodes(), args[0])
	if err != nil {
		n.Ui.Error(err.Error())
		return 1
	}

	req := api.NodeIdentityGetRequest{NodeID: nodeID}

	resp, err := client.Nodes().Identity().Get(&req, nil)
	if err != nil {
		n.Ui.Error(fmt.Sprintf("Error requesting node identity: %s", err))
		return 1
	}

	return n.ouputClaims(resp.Claims)
}

func (n *NodeIdentityGetCommand) ouputClaims(claims map[string]any) int {

	// If the user has requested JSON output or a template, format the claims
	// accordingly.
	if n.json || len(n.tmpl) > 0 {
		out, err := Format(n.json, n.tmpl, claims)
		if err != nil {
			n.Ui.Error(err.Error())
			return 1
		}

		n.Ui.Output(out)
		return 0
	}

	var genericClaims, nomadClaims []string

	// Iterate through the claims and separate the generic and Nomad-specific
	// claims. This will allow us to group them in the output.
	for key := range claims {
		if strings.HasPrefix(key, "nomad") {
			nomadClaims = append(nomadClaims, key)
		} else {
			genericClaims = append(genericClaims, key)
		}
	}

	// Sort the claims alphabetically for consistent output.
	sort.Strings(genericClaims)
	sort.Strings(nomadClaims)

	output := make([]string, len(genericClaims)+len(nomadClaims)+1)
	output[0] = "Claim Key|Claim Value"

	for i, key := range genericClaims {

		// The generic claims currently include timestamps which come to the CLI
		// as float64 values. We need to correctly convert these into a
		// human-readable format. All other claims are string values.
		switch valT := claims[key].(type) {
		case float64:
			output[i+1] = fmt.Sprintf("%s | %v", key, formatTime(time.Unix(int64(valT), 0)))
		default:
			output[i+1] = fmt.Sprintf("%s | %s", key, valT)
		}
	}

	for i, key := range nomadClaims {
		output[i+1+len(genericClaims)] = fmt.Sprintf("%s | %s", key, claims[key])
	}

	n.Ui.Output(formatList(output))
	return 0
}

func (n *NodeIdentityGetCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(n.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-json": complete.PredictNothing,
			"-t":    complete.PredictAnything,
		})
}

func (n *NodeIdentityGetCommand) AutocompleteArgs() complete.Predictor {
	return nodePredictor(n.Client, nil)
}

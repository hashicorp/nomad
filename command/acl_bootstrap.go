// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
)

type ACLBootstrapCommand struct {
	Meta
}

func (c *ACLBootstrapCommand) Help() string {
	helpText := `
Usage: nomad acl bootstrap [options]

  Bootstrap is used to bootstrap the ACL system and get an initial token.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

Bootstrap Options:

  -json
    Output the bootstrap response in JSON format.

  -t
    Format and display the bootstrap response using a Go template.

`
	return strings.TrimSpace(helpText)
}

func (c *ACLBootstrapCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-json": complete.PredictNothing,
			"-t":    complete.PredictAnything,
		})
}

func (c *ACLBootstrapCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *ACLBootstrapCommand) Synopsis() string {
	return "Bootstrap the ACL system for initial token"
}

func (c *ACLBootstrapCommand) Name() string { return "acl bootstrap" }

func (c *ACLBootstrapCommand) Run(args []string) int {

	var (
		json bool
		tmpl string
		file string
	)

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&json, "json", false, "")
	flags.StringVar(&tmpl, "t", "", "")
	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got no arguments
	args = flags.Args()
	if l := len(args); l < 0 || l > 1 {
		c.Ui.Error("This command takes up to one argument")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	var terminalToken []byte
	var err error

	if len(args) == 1 {
		switch args[0] {
		case "":
			terminalToken = []byte{}
		case "-":
			terminalToken, err = io.ReadAll(os.Stdin)
		default:
			file = args[0]
			terminalToken, err = os.ReadFile(file)
		}
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error reading provided token: %v", err))
			return 1
		}
	}

	// Remove newline from the token if it was passed by stdin
	boottoken := strings.TrimSuffix(string(terminalToken), "\n")

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Get the bootstrap token
	token, _, err := client.ACLTokens().BootstrapOpts(boottoken, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error bootstrapping: %s", err))
		return 1
	}

	if json || len(tmpl) > 0 {
		out, err := Format(json, tmpl, token)
		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}

		c.Ui.Output(out)
		return 0
	}

	// Format the output
	outputACLToken(c.Ui, token)
	return 0
}

// formatACLPolicy returns formatted policy
func formatACLPolicy(policy *api.ACLPolicy) string {
	output := []string{
		fmt.Sprintf("Name|%s", policy.Name),
		fmt.Sprintf("Description|%s", policy.Description),
		fmt.Sprintf("CreateIndex|%v", policy.CreateIndex),
		fmt.Sprintf("ModifyIndex|%v", policy.ModifyIndex),
	}

	formattedOut := formatKV(output)

	if policy.JobACL != nil {
		output := []string{
			fmt.Sprintf("Namespace|%v", policy.JobACL.Namespace),
			fmt.Sprintf("JobID|%v", policy.JobACL.JobID),
			fmt.Sprintf("Group|%v", policy.JobACL.Group),
			fmt.Sprintf("Task|%v", policy.JobACL.Task),
		}
		formattedOut += "\n\n[bold]Associated Workload[reset]\n"
		formattedOut += formatKV(output)
	}

	// these are potentially large blobs so leave till the end
	formattedOut += "\n\n[bold]Rules[reset]\n\n"
	formattedOut += policy.Rules

	return formattedOut
}

// outputACLToken formats and outputs the ACL token via the UI in the correct
// format.
func outputACLToken(ui cli.Ui, token *api.ACLToken) {

	// Build the initial KV output which is always the same not matter whether
	// the token is a management or client type.
	kvOutput := []string{
		fmt.Sprintf("Accessor ID|%s", token.AccessorID),
		fmt.Sprintf("Secret ID|%s", token.SecretID),
		fmt.Sprintf("Name|%s", token.Name),
		fmt.Sprintf("Type|%s", token.Type),
		fmt.Sprintf("Global|%v", token.Global),
		fmt.Sprintf("Create Time|%v", token.CreateTime),
		fmt.Sprintf("Expiry Time |%s", expiryTimeString(token.ExpirationTime)),
		fmt.Sprintf("Create Index|%d", token.CreateIndex),
		fmt.Sprintf("Modify Index|%d", token.ModifyIndex),
	}

	// If the token is a management type, make it obvious that it is not
	// possible to have policies or roles assigned to it and just output the
	// KV data.
	if token.Type == "management" {
		kvOutput = append(kvOutput, "Policies|n/a", "Roles|n/a")
		ui.Output(formatKV(kvOutput))
	} else {

		// Policies are only currently referenced by name, so keep the previous
		// format. When/if policies gain an ID alongside name like roles, this
		// output should follow that of the roles.
		kvOutput = append(kvOutput, fmt.Sprintf("Policies|%v", token.Policies))

		var roleOutput []string

		// If we have linked roles, add the ID and name in a list format to the
		// output. Otherwise, make it clear there are no linked roles.
		if len(token.Roles) > 0 {
			roleOutput = append(roleOutput, "ID|Name")
			for _, roleLink := range token.Roles {
				roleOutput = append(roleOutput, roleLink.ID+"|"+roleLink.Name)
			}
		} else {
			roleOutput = append(roleOutput, "<none>")
		}

		// Output the mixed formats of data, ensuring there is a space between
		// the KV and list data.
		ui.Output(formatKV(kvOutput))
		ui.Output("")
		ui.Output(fmt.Sprintf("Roles\n%s", formatList(roleOutput)))
	}
}

func expiryTimeString(t *time.Time) string {
	if t == nil || t.IsZero() {
		return "<none>"
	}
	return t.String()
}

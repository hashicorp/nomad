// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/cli"
	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

// ensure interface satisfaction
var _ cli.Command = &VolumeClaimListCommand{}

type VolumeClaimListCommand struct {
	Meta

	job        string
	taskGroup  string
	volumeName string

	json bool
	tmpl string
}

func (c *VolumeClaimListCommand) Help() string {
	helpText := `
Usage: nomad volume claim list [options]

  volume claim list is used to list existing host volume claims.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

List Options:

  -job <id>
    Filter volume claims by job ID.

  -group <name>
    Filter volumes claims by task-group name.

  -volume-name <name>
    Filter volumes claims by volume name.

  -json
    Output the host volume claims in a JSON format.
  -t
    Format and display the host volume claims using a Go template.
`
	return strings.TrimSpace(helpText)
}

func (c *VolumeClaimListCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-job":         complete.PredictNothing,
			"-group":       complete.PredictNothing,
			"-volume-name": complete.PredictNothing,
			"-json":        complete.PredictNothing,
			"-t":           complete.PredictAnything,
		})
}

func (c *VolumeClaimListCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *VolumeClaimListCommand) Name() string {
	return "volume claim list"
}

func (c *VolumeClaimListCommand) Synopsis() string {
	return "List existing host volume claims"
}

func (c *VolumeClaimListCommand) Run(args []string) int {
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.StringVar(&c.job, "job", "", "")
	flags.StringVar(&c.taskGroup, "group", "", "")
	flags.StringVar(&c.volumeName, "volume-name", "", "")
	flags.BoolVar(&c.json, "json", false, "")
	flags.StringVar(&c.tmpl, "t", "", "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got no arguments
	if len(flags.Args()) != 0 {
		c.Ui.Error("This command takes no arguments")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	claims, _, err := client.TaskGroupHostVolumeClaims().List(&api.TaskGroupHostVolumeClaimsListRequest{
		JobID:      c.job,
		TaskGroup:  c.taskGroup,
		VolumeName: c.volumeName,
	}, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error listing task group host volume claims: %s", err))
		return 1
	}

	if c.json || len(c.tmpl) > 0 {
		out, err := Format(c.json, c.tmpl, claims)
		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}

		c.Ui.Output(out)
		return 0
	}

	c.Ui.Output(formatClaims(claims))
	return 0
}

func formatClaims(claims []*api.TaskGroupHostVolumeClaim) string {
	if len(claims) == 0 {
		return "No task group host volume claims found"
	}

	output := make([]string, 0, len(claims)+1)
	output = append(output, "ID|Namespace|Job ID|Volume ID|Volume Name")
	for _, claim := range claims {
		output = append(output, fmt.Sprintf(
			"%s|%s|%s|%s|%s",
			claim.ID, claim.Namespace, claim.JobID, claim.VolumeID, claim.VolumeName))
	}

	return formatList(output)
}

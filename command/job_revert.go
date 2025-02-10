// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/api/contexts"
	"github.com/posener/complete"
)

type JobRevertCommand struct {
	Meta
}

func (c *JobRevertCommand) Help() string {
	helpText := `
Usage: nomad job revert [options] <job> <version|tag>

  Revert is used to revert a job to a prior version of the job. The available
  versions to revert to can be found using "nomad job history" command.

  When ACLs are enabled, this command requires a token with the 'submit-job'
  capability for the job's namespace. The 'list-jobs' capability is required to
  run the command with a job prefix instead of the exact job ID. The 'read-job'
  capability is required to monitor the resulting evaluation when -detach is
  not used.

  If the version number is specified, the job will be reverted to the exact
  version number. If a version tag is specified, the job will be reverted to
  the version with the given tag.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Revert Options:

  -detach
    Return immediately instead of entering monitor mode. After job revert,
    the evaluation ID will be printed to the screen, which can be used to
    examine the evaluation using the eval-status command.

  -consul-token
   The Consul token used to verify that the caller has access to the Service
   Identity policies associated in the targeted version of the job.

  -vault-token
   The Vault token used to verify that the caller has access to the Vault
   policies in the targeted version of the job.

  -verbose
    Display full information.
`
	return strings.TrimSpace(helpText)
}

func (c *JobRevertCommand) Synopsis() string {
	return "Revert to a prior version of the job"
}

func (c *JobRevertCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-detach":  complete.PredictNothing,
			"-verbose": complete.PredictNothing,
		})
}

func (c *JobRevertCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) []string {
		client, err := c.Meta.Client()
		if err != nil {
			return nil
		}

		resp, _, err := client.Search().PrefixSearch(a.Last, contexts.Jobs, nil)
		if err != nil {
			return []string{}
		}
		return resp.Matches[contexts.Jobs]
	})
}

func (c *JobRevertCommand) Name() string { return "job revert" }

func (c *JobRevertCommand) Run(args []string) int {
	var detach, verbose bool
	var consulToken, vaultToken string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&detach, "detach", false, "")
	flags.BoolVar(&verbose, "verbose", false, "")
	flags.StringVar(&consulToken, "consul-token", "", "")
	flags.StringVar(&vaultToken, "vault-token", "", "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Truncate the id unless full length is requested
	length := shortId
	if verbose {
		length = fullId
	}

	// Check that we got two args
	args = flags.Args()
	if l := len(args); l != 2 {
		c.Ui.Error("This command takes two arguments: <job> <version|tag>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Parse the Consul token
	if consulToken == "" {
		// Check the environment variable
		consulToken = os.Getenv("CONSUL_HTTP_TOKEN")
	}

	// Parse the Vault token
	if vaultToken == "" {
		// Check the environment variable
		vaultToken = os.Getenv("VAULT_TOKEN")
	}

	// Parse the job version or version tag
	var revertVersion uint64

	parsedVersion, ok, err := parseVersion(args[1])
	if ok && err == nil {
		revertVersion = parsedVersion
	} else {
		foundTaggedVersion, _, err := client.Jobs().VersionByTag(args[0], args[1], nil)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error retrieving job versions: %s", err))
			return 1
		}
		revertVersion = *foundTaggedVersion.Version
	}

	// Check if the job exists
	jobIDPrefix := strings.TrimSpace(args[0])
	jobID, namespace, err := c.JobIDByPrefix(client, jobIDPrefix, nil)
	if err != nil {
		c.Ui.Error(err.Error())
		return 1
	}

	// Prefix lookup matched a single job
	q := &api.WriteOptions{Namespace: namespace}
	resp, _, err := client.Jobs().Revert(jobID, revertVersion, nil, q, consulToken, vaultToken)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error retrieving job versions: %s", err))
		return 1
	}

	// Nothing to do
	evalCreated := resp.EvalID != ""

	if !evalCreated {
		return 0
	}

	if detach {
		c.Ui.Output("Evaluation ID: " + resp.EvalID)
		return 0
	}

	mon := newMonitor(c.Ui, client, length)
	return mon.monitor(resp.EvalID)
}

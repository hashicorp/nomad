// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/command/asset"
	"github.com/posener/complete"
)

const (
	// DefaultInitName is the default name we use when
	// initializing the example file
	DefaultInitName = "example.nomad.hcl"
)

// JobInitCommand generates a new job template that you can customize to your
// liking, like vagrant init
type JobInitCommand struct {
	Meta
}

func (c *JobInitCommand) Help() string {
	helpText := `
Usage: nomad job init <filename>
Alias: nomad init <filename>

  Creates an example job file that can be used as a starting point to customize
  further. If no filename is given, the default of "example.nomad.hcl" will be used.

Init Options:

  -short
    If the short flag is set, a minimal jobspec without comments is emitted.

  -connect
    If the connect flag is set, the jobspec includes Consul Connect integration.

  -template
    Specifies a predefined template to initialize. Must be a Nomad Variable that
    lives at nomad/job-templates/<template>

  -list-templates
    Display a list of possible job templates to pass to -template. Reads from
    all variables pathed at nomad/job-templates/<template>
`
	return strings.TrimSpace(helpText)
}

func (c *JobInitCommand) Synopsis() string {
	return "Create an example job file"
}

func (c *JobInitCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-short": complete.PredictNothing,
		})
}

func (c *JobInitCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *JobInitCommand) Name() string { return "job init" }

func (c *JobInitCommand) Run(args []string) int {
	var short bool
	var connect bool
	var template string
	var listTemplates bool

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&short, "short", false, "")
	flags.BoolVar(&connect, "connect", false, "")
	flags.StringVar(&template, "template", "", "The name of the job template variable to initialize")
	flags.BoolVar(&listTemplates, "list-templates", false, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check for misuse
	// Check that we either got no filename or exactly one.
	args = flags.Args()
	if len(args) > 1 {
		c.Ui.Error("This command takes either no arguments or one: <filename>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	filename := DefaultInitName
	if len(args) == 1 {
		filename = args[0]
	}

	// Check if the file already exists
	_, err := os.Stat(filename)
	if err != nil && !os.IsNotExist(err) {
		c.Ui.Error(fmt.Sprintf("Failed to stat '%s': %v", filename, err))
		return 1
	}
	if !os.IsNotExist(err) && !listTemplates {
		c.Ui.Error(fmt.Sprintf("Job '%s' already exists", filename))
		return 1
	}

	var jobSpec []byte

	if listTemplates {
		// Get the HTTP client
		client, err := c.Meta.Client()
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
			return 1
		}
		qo := &api.QueryOptions{
			Namespace: c.Meta.namespace,
		}

		// Get and list all variables at nomad/job-templates
		vars, _, err := client.Variables().PrefixList("nomad/job-templates/", qo)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error retrieving job templates from the server; unable to read variables at path nomad/job-templates/. Error: %s", err))
			return 1
		}

		if len(vars) == 0 {
			c.Ui.Error("No variables in nomad/job-templates")
			return 1
		} else {
			c.Ui.Output("Use nomad job init -template=<template> with any of the following:")
			for _, v := range vars {
				c.Ui.Output(fmt.Sprintf("  %s", strings.TrimPrefix(v.Path, "nomad/job-templates/")))
			}
		}
		return 0
	} else if template != "" {
		// Get the HTTP client
		client, err := c.Meta.Client()
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error initializing: %s", err))
			return 1
		}

		qo := &api.QueryOptions{
			Namespace: c.Meta.namespace,
		}
		sv, _, err := client.Variables().Read("nomad/job-templates/"+template, qo)
		if err != nil {
			if err.Error() == "variable not found" {
				c.Ui.Warn(errVariableNotFound)
				return 1
			}
			c.Ui.Error(fmt.Sprintf("Error retrieving variable: %s", err))
			return 1
		}

		if v, ok := sv.Items["template"]; ok {
			c.Ui.Output(fmt.Sprintf("Initializing a job template from %s", template))
			jobSpec = []byte(v)
		} else {
			c.Ui.Error(fmt.Sprintf("Job template %q is malformed and is missing a template field. Please visit the jobs/run/templates route in  the Nomad UI to add it", template))
			return 1
		}

	} else {
		switch {
		case connect && !short:
			jobSpec = asset.JobConnect
		case connect && short:
			jobSpec = asset.JobConnectShort
		case !connect && short:
			jobSpec = asset.JobExampleShort
		default:
			jobSpec = asset.JobExample
		}
	}

	// Write out the example
	err = os.WriteFile(filename, jobSpec, 0660)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to write '%s': %v", filename, err))
		return 1
	}

	// Success
	c.Ui.Output(fmt.Sprintf("Example job file written to %s", filename))
	return 0
}

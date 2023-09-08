// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"os"
	"strings"

	"github.com/posener/complete"
)

const (
	// DefaultHclQuotaInitName is the default name we use when initializing the
	// example quota file in HCL format
	DefaultHclQuotaInitName = "spec.hcl"

	// DefaultHclQuotaInitName is the default name we use when initializing the
	// example quota file in JSON format
	DefaultJsonQuotaInitName = "spec.json"
)

// QuotaInitCommand generates a new quota spec that you can customize to your
// liking, like vagrant init
type QuotaInitCommand struct {
	Meta
}

func (c *QuotaInitCommand) Help() string {
	helpText := `
Usage: nomad quota init <filename>

  Creates an example quota specification file that can be used as a starting
  point to customize further. If no filename is given, the default of "spec.hcl"
  or "spec.json" will be used.

Init Options:

  -json
    Create an example JSON quota specification.
`
	return strings.TrimSpace(helpText)
}

func (c *QuotaInitCommand) Synopsis() string {
	return "Create an example quota specification file"
}

func (c *QuotaInitCommand) AutocompleteFlags() complete.Flags {
	return complete.Flags{
		"-json": complete.PredictNothing,
	}
}

func (c *QuotaInitCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *QuotaInitCommand) Name() string { return "quota init" }

func (c *QuotaInitCommand) Run(args []string) int {
	var jsonOutput bool
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&jsonOutput, "json", false, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we get no arguments
	args = flags.Args()
	if l := len(args); l > 1 {
		c.Ui.Error("This command takes no arguments or one: <filename>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	fileName := DefaultHclQuotaInitName
	fileContent := defaultHclQuotaSpec
	if jsonOutput {
		fileName = DefaultJsonQuotaInitName
		fileContent = defaultJsonQuotaSpec
	}
	if len(args) == 1 {
		fileName = args[0]
	}

	// Check if the file already exists
	_, err := os.Stat(fileName)
	if err != nil && !os.IsNotExist(err) {
		c.Ui.Error(fmt.Sprintf("Failed to stat %q: %v", fileName, err))
		return 1
	}
	if !os.IsNotExist(err) {
		c.Ui.Error(fmt.Sprintf("Quota specification %q already exists", fileName))
		return 1
	}

	// Write out the example
	err = os.WriteFile(fileName, []byte(fileContent), 0660)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to write %q: %v", fileName, err))
		return 1
	}

	// Success
	c.Ui.Output(fmt.Sprintf("Example quota specification written to %s", fileName))
	return 0
}

var defaultHclQuotaSpec = strings.TrimSpace(`
name        = "default-quota"
description = "Limit the shared default namespace"

# Create a limit for the global region. Additional limits may
# be specified in-order to limit other regions.
limit {
  region = "global"
  region_limit {
    cpu        = 2500
    memory     = 1000
    memory_max = 1000
  }
  variables_limit = 1000
}
`)

var defaultJsonQuotaSpec = strings.TrimSpace(`
{
	"Name": "default-quota",
	"Description": "Limit the shared default namespace",
	"Limits": [
		{
			"Region": "global",
			"RegionLimit": {
				"CPU": 2500,
				"MemoryMB": 1000,
				"MemoryMaxMB": 1000
			},
			"VariablesLimit": 1000
		}
	]
}
`)

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package command

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/posener/complete"
)

const (
	// DefaultHclNodePoolInitName is the default name we use when initializing
	// the example node pool spec file in HCL format
	DefaultHclNodePoolInitName = "pool.np.hcl"

	// DefaultJsonNodePoolInitName is the default name we use when initializing
	// the example node pool spec in JSON format
	DefaultJsonNodePoolInitName = "pool.np.json"
)

// NodePoolInitCommand generates a new variable specification
type NodePoolInitCommand struct {
	Meta
}

func (c *NodePoolInitCommand) Help() string {
	helpText := `
Usage: nomad node pool init <filename>

  Creates an example node pool specification file that can be used as a starting
  point to customize further. When no filename is supplied, a default filename
  of "pool.np.hcl" or "pool.np.json" will be used depending on the output
  format.

Init Options:

  -out (hcl | json)
    Format of generated node pool specification. Defaults to "hcl".

`
	return strings.TrimSpace(helpText)
}

func (c *NodePoolInitCommand) Synopsis() string {
	return "Create an example node pool specification file"
}

func (c *NodePoolInitCommand) AutocompleteFlags() complete.Flags {
	return complete.Flags{
		"-out": complete.PredictSet("hcl", "json"),
	}
}

func (c *NodePoolInitCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *NodePoolInitCommand) Name() string { return "node pool init" }

func (c *NodePoolInitCommand) Run(args []string) int {
	var outFmt string
	var quiet bool

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.StringVar(&outFmt, "out", "hcl", "")

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
	var fileName, fileContent string
	switch outFmt {
	case "hcl":
		fileName = DefaultHclNodePoolInitName
		fileContent = defaultHclNodePoolSpec
	case "json":
		fileName = DefaultJsonNodePoolInitName
		fileContent = defaultJsonNodePoolSpec
	}

	if len(args) == 1 {
		fileName = args[0]
	}

	// Check if the file already exists
	_, err := os.Stat(fileName)
	if err == nil {
		c.Ui.Error(fmt.Sprintf("File %q already exists", fileName))
		return 1
	}
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		c.Ui.Error(fmt.Sprintf("Failed to stat %q: %v", fileName, err))
		return 1
	}

	// Write out the example
	err = os.WriteFile(fileName, []byte(fileContent), 0660)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to write %q: %v", fileName, err))
		return 1
	}

	// Success
	if !quiet {
		c.Ui.Output(fmt.Sprintf("Example node pool specification written to %s", fileName))
	}
	return 0
}

var defaultHclNodePoolSpec = strings.TrimSpace(`
node_pool "example" {

  description = "Example node pool"

  # meta is optional metadata on the node pool, defined as key-value pairs.
  # The scheduler does not use node pool metadata as part of scheduling.
  meta {
    environment = "prod"
    owner       = "sre"
  }

  # The scheduler configuration options specific to this node pool. This block
  # supports a subset of the fields supported in the global scheduler
  # configuration as described at:
  # https://developer.hashicorp.com/nomad/docs/commands/operator/scheduler/set-config
  #
  # Available only in Nomad Enterprise.
  scheduler_configuration {

    # scheduler_algorithm is the scheduling algorithm to use for the pool.
    # If not defined, the global cluster scheduling algorithm is used.
    scheduler_algorithm = "spread"
  }
}
`) + "\n"

var defaultJsonNodePoolSpec = strings.TrimSpace(`
{
  "Name": "example",
  "Description": "Example node pool",
  "Meta": {
    "environment": "prod",
    "owner": "sre"
  },
  "SchedulerConfiguration": {
    "SchedulerAlgorithm": "spread"
  }
}
`) + "\n"

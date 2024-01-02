// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/posener/complete"
)

type VolumeCreateCommand struct {
	Meta
}

func (c *VolumeCreateCommand) Help() string {
	helpText := `
Usage: nomad volume create [options] <input>

  Creates a volume in an external storage provider and registers it in Nomad.

  If the supplied path is "-" the volume file is read from stdin. Otherwise, it
  is read from the file at the supplied path.

  When ACLs are enabled, this command requires a token with the
  'csi-write-volume' capability for the volume's namespace.

General Options:

  ` + generalOptionsUsage(usageOptsDefault)

	return strings.TrimSpace(helpText)
}

func (c *VolumeCreateCommand) AutocompleteFlags() complete.Flags {
	return c.Meta.AutocompleteFlags(FlagSetClient)
}

func (c *VolumeCreateCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictFiles("*")
}

func (c *VolumeCreateCommand) Synopsis() string {
	return "Create an external volume"
}

func (c *VolumeCreateCommand) Name() string { return "volume create" }

func (c *VolumeCreateCommand) Run(args []string) int {
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }

	if err := flags.Parse(args); err != nil {
		c.Ui.Error(fmt.Sprintf("Error parsing arguments %s", err))
		return 1
	}

	// Check that we get exactly one argument
	args = flags.Args()
	if l := len(args); l != 1 {
		c.Ui.Error("This command takes one argument: <input>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	// Read the file contents
	file := args[0]
	var rawVolume []byte
	var err error
	if file == "-" {
		rawVolume, err = io.ReadAll(os.Stdin)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Failed to read stdin: %v", err))
			return 1
		}
	} else {
		rawVolume, err = os.ReadFile(file)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Failed to read file: %v", err))
			return 1
		}
	}

	ast, volType, err := parseVolumeType(string(rawVolume))
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error parsing the volume type: %s", err))
		return 1
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	switch strings.ToLower(volType) {
	case "csi":
		code := c.csiCreate(client, ast)
		return code
	default:
		c.Ui.Error(fmt.Sprintf("Error unknown volume type: %s", volType))
		return 1
	}
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/nomad/command/asset"
	"github.com/posener/complete"
)

const (
	// defaultHclVolumeInitName is the default name we use when initializing
	// the example volume file in HCL format
	defaultHclVolumeInitName = "volume.hcl"

	// DefaultHclVolumeInitName is the default name we use when initializing
	// the example volume file in JSON format
	defaultJsonVolumeInitName = "volume.json"
)

// VolumeInitCommand generates a new volume spec that you can customize to
// your liking, like vagrant init
type VolumeInitCommand struct {
	Meta
}

func (c *VolumeInitCommand) Help() string {
	helpText := `
Usage: nomad volume init <filename>

  Creates an example volume specification file that can be used as a starting
  point to customize further. If no filename is give, the default "volume.json"
  or "volume.hcl" will be used.

Init Options:

  -json
    Create an example JSON volume specification.

  -type
    Create an example for a specific type of volume (one of "csi" or "host",
    defaults to "csi").

`
	return strings.TrimSpace(helpText)
}

func (c *VolumeInitCommand) Synopsis() string {
	return "Create an example volume specification file"
}

func (c *VolumeInitCommand) AutocompleteFlags() complete.Flags {
	return complete.Flags{
		"-json": complete.PredictNothing,
		"-type": complete.PredictSet("host", "csi"),
	}
}

func (c *VolumeInitCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *VolumeInitCommand) Name() string { return "volume init" }

func (c *VolumeInitCommand) Run(args []string) int {
	var jsonOutput bool
	var volType string
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&jsonOutput, "json", false, "")
	flags.StringVar(&volType, "type", "csi", "type of volume")

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

	fileName := defaultHclVolumeInitName
	fileContent := asset.CSIVolumeSpecHCL

	if volType == "host" && !jsonOutput {
		fileContent = asset.HostVolumeSpecHCL
	} else if volType == "host" && jsonOutput {
		fileName = defaultJsonVolumeInitName
		fileContent = asset.HostVolumeSpecJSON
	} else if jsonOutput {
		fileName = defaultJsonVolumeInitName
		fileContent = asset.CSIVolumeSpecJSON
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
		c.Ui.Error(fmt.Sprintf("Volume specification %q already exists", fileName))
		return 1
	}

	// Write out the example
	err = os.WriteFile(fileName, fileContent, 0660)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to write %q: %v", fileName, err))
		return 1
	}

	// Success
	c.Ui.Output(fmt.Sprintf("Example volume specification written to %s", fileName))
	return 0
}

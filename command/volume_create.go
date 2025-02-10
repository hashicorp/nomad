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

  When ACLs are enabled, this command requires a token with the appropriate
  capability in the volume's namespace: the 'csi-write-volume' capability for
  CSI volumes or 'host-volume-create' for dynamic host volumes.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Create Options:

  -detach
    Return immediately instead of entering monitor mode for dynamic host
    volumes. After creating a volume, the volume ID will be printed to the
    screen, which can be used to examine the volume using the volume status
    command. If -detach is omitted or false, the command will monitor the state
    of the volume until it is ready to be scheduled.

  -id
    Update a volume previously created with this ID prefix. Used for dynamic
    host volumes only.

  -verbose
    Display full information when monitoring volume state. Used for dynamic host
    volumes only.

  -policy-override
    Sets the flag to force override any soft mandatory Sentinel policies. Used
    for dynamic host volumes only.
`

	return strings.TrimSpace(helpText)
}

func (c *VolumeCreateCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-detach":          complete.PredictNothing,
			"-verbose":         complete.PredictNothing,
			"-policy-override": complete.PredictNothing,
			"-id":              complete.PredictNothing,
		})
}

func (c *VolumeCreateCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictFiles("*")
}

func (c *VolumeCreateCommand) Synopsis() string {
	return "Create an external volume"
}

func (c *VolumeCreateCommand) Name() string { return "volume create" }

func (c *VolumeCreateCommand) Run(args []string) int {
	var detach, verbose, override bool
	var volID string
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.BoolVar(&detach, "detach", false, "detach from monitor")
	flags.BoolVar(&verbose, "verbose", false, "display full volume IDs")
	flags.BoolVar(&override, "policy-override", false, "override soft mandatory Sentinel policies")
	flags.StringVar(&volID, "id", "", "update an existing dynamic host volume")
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
		return c.csiCreate(client, ast)
	case "host":
		return c.hostVolumeCreate(client, ast, detach, verbose, override, volID)
	default:
		c.Ui.Error(fmt.Sprintf("Error unknown volume type: %s", volType))
		return 1
	}
}

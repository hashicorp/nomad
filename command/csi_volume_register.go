package command

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/hashicorp/hcl"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/helper"
	"github.com/posener/complete"
)

type CSIVolumeRegisterCommand struct {
	Meta
}

func (c *CSIVolumeRegisterCommand) Help() string {
	helpText := `
Usage: nomad csi volume register [options] <input>

  Register is used to create or update a CSI volume specification. The volume
  must exist on the remote storage provider before it can be used by a task.
  The specification file will be read from stdin by specifying "-", otherwise
  a path to the file is expected. The specification may be in HCL or JSON.

General Options:

  ` + generalOptionsUsage() + `

Register Options:
`

	return strings.TrimSpace(helpText)
}

func (c *CSIVolumeRegisterCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-json": complete.PredictNothing,
		})
}

func (c *CSIVolumeRegisterCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictFiles("*")
}

func (c *CSIVolumeRegisterCommand) Synopsis() string {
	return "Create or update a CSI volume specification"
}

func (c *CSIVolumeRegisterCommand) Name() string { return "csi volume register" }

func (c *CSIVolumeRegisterCommand) Run(args []string) int {
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }

	if err := flags.Parse(args); err != nil {
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
		rawVolume, err = ioutil.ReadAll(os.Stdin)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Failed to read stdin: %v", err))
			return 1
		}
	} else {
		rawVolume, err = ioutil.ReadFile(file)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Failed to read file: %v", err))
			return 1
		}
	}

	vol, err := parseCSIVolume(string(rawVolume))
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error parsing the volume: %s", err))
		return 1
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	_, err = client.CSIVolumes().Register(vol, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error registering volume: %s", err))
		return 1
	}

	// c.Ui.Output(fmt.Sprintf("Successfully applied quota specification %q!", spec.Name))
	return 0
}

// parseCSIVolume is used to parse the quota specification from HCL
func parseCSIVolume(input string) (*api.CSIVolume, error) {
	output := &api.CSIVolume{}
	err := hcl.Decode(output, input)
	if err != nil {
		return nil, err
	}

	err = helper.UnusedKeys(output)
	if err != nil {
		return nil, err
	}

	return output, nil
}

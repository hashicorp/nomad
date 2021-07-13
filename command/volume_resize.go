package command

import (
	"fmt"
	"strings"

	humanize "github.com/dustin/go-humanize"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/api/contexts"
	"github.com/posener/complete"
)

type VolumeResizeCommand struct {
	Meta
}

func (c *VolumeResizeCommand) Help() string {
	helpText := `
Usage: nomad volume resize [options] <volume id> [-min size] [-max size]

  Resize a CSI volume. The size arguments accept human-friendly suffixes such
  as "100GiB". The volume must be registered with Nomad in order to be
  resized. Resizing will fail if the volume is still in use by an allocation
  or in the process of being unpublished, if the plugin does not support
  online resize.

  When ACLs are enabled, this command requires a token with the
  'csi-write-volume' capability for the volume's namespace.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Resize Options:

  -min: Option for setting the capacity. The new volume size must be at least
   this large, in bytes. The storage provider may return a volume that is
   larger than this value. Accepts human-friendly suffixes such as '"100GiB"'.

  -max: Option for setting the capacity. The volume must be no more than this
   large, in bytes. The storage provider may return a volume that is smaller
   than this value. Accepts human-friendly suffixes such as '"100GiB"'.

`
	return strings.TrimSpace(helpText)
}

func (c *VolumeResizeCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{})
}

func (c *VolumeResizeCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) []string {
		client, err := c.Meta.Client()
		if err != nil {
			return nil
		}

		resp, _, err := client.Search().PrefixSearch(a.Last, contexts.Volumes, nil)
		if err != nil {
			return []string{}
		}
		return resp.Matches[contexts.Volumes]
	})
}

func (c *VolumeResizeCommand) Synopsis() string {
	return "Resize a volume"
}

func (c *VolumeResizeCommand) Name() string { return "volume resize" }

func (c *VolumeResizeCommand) Run(args []string) int {
	var minFlag string
	var maxFlag string
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.StringVar(&minFlag, "min", "", "")
	flags.StringVar(&maxFlag, "max", "", "")

	if err := flags.Parse(args); err != nil {
		c.Ui.Error(fmt.Sprintf("Error parsing arguments %s", err))
		return 1
	}
	// Check that we get exactly two arguments
	args = flags.Args()
	if l := len(args); l != 1 {
		c.Ui.Error("This command takes one argument: <volume id>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}
	volID := args[0]

	var minSize int64
	var maxSize int64

	if minFlag != "" {
		b, err := humanize.ParseBytes(minFlag)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("invalid min value: %v", err))
			return 1
		}
		minSize = int64(b)
	}
	if maxFlag != "" {
		b, err := humanize.ParseBytes(maxFlag)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("invalid max value: %v", err))
			return 1
		}
		maxSize = int64(b)
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	resp, _, err := client.CSIVolumes().Resize(&api.CSIVolumeResizeRequest{
		VolumeID:             volID,
		RequestedCapacityMin: minSize,
		RequestedCapacityMax: maxSize,
	}, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error resizing volume: %s", err))
		return 1
	}
	if resp.CapacityBytes < 0 {
		c.Ui.Error(fmt.Sprintf("Plugin returned invalid capacity %v", resp.CapacityBytes))
		return 1
	}

	c.Ui.Output(fmt.Sprintf("Resized volume %s to %s", volID,
		humanize.IBytes(uint64(resp.CapacityBytes))))
	return 0
}

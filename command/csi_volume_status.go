package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/api/contexts"
	"github.com/posener/complete"
)

type CSIVolumeStatusCommand struct {
	Meta
	length    int
	evals     bool
	allAllocs bool
	verbose   bool
}

func (c *CSIVolumeStatusCommand) Help() string {
	helpText := `
Usage: nomad csi volume status [options] <id>

  Display status information about a CSI volume. If no volume id is given, a
  list of all volumes will be displayed.

General Options:

  ` + generalOptionsUsage() + `

Status Options:

  -short
    Display short output. Used only when a single volume is being
    queried, and drops verbose information about allocations.

  -evals
    Display the evaluations using the volume.

  -all-allocs
    Display all allocations using the volume, including those that are no
    longer running.

  -verbose
    Display full information.
`
	return strings.TrimSpace(helpText)
}

func (c *CSIVolumeStatusCommand) Synopsis() string {
	return "Display status information about a volume"
}

func (c *CSIVolumeStatusCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-all-allocs": complete.PredictNothing,
			"-evals":      complete.PredictNothing,
			"-short":      complete.PredictNothing,
			"-verbose":    complete.PredictNothing,
		})
}

func (c *CSIVolumeStatusCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) []string {
		client, err := c.Meta.Client()
		if err != nil {
			return nil
		}

		resp, _, err := client.Search().PrefixSearch(a.Last, contexts.CSIVolumes, nil)
		if err != nil {
			return []string{}
		}
		return resp.Matches[contexts.CSIVolumes]
	})
}

func (c *CSIVolumeStatusCommand) Name() string { return "csi volume status" }

func (c *CSIVolumeStatusCommand) Run(args []string) int {
	var short bool

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&short, "short", false, "")
	flags.BoolVar(&c.evals, "evals", false, "")
	flags.BoolVar(&c.allAllocs, "all-allocs", false, "")
	flags.BoolVar(&c.verbose, "verbose", false, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we either got no jobs or exactly one.
	args = flags.Args()
	if len(args) > 1 {
		c.Ui.Error("This command takes either no arguments or one: <volume id>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	// Truncate the id unless full length is requested
	c.length = shortId
	if c.verbose {
		c.length = fullId
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Invoke list mode if no job ID.
	if len(args) == 0 {
		vols, _, err := client.CSIVolumes().List(nil)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error querying jobs: %s", err))
			return 1
		}

		if len(vols) == 0 {
			// No output if we have no jobs
			c.Ui.Output("No CSI volumes")
		} else {
			c.Ui.Output(formatCSIVolumeList(vols))
		}
		return 0
	}

	// Try querying the job
	volID := args[0]

	// Lookup matched a single job
	vol, _, err := client.CSIVolumes().Info(volID, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying volume: %s", err))
		return 1
	}

	c.Ui.Output(c.formatBasic(vol))

	// Exit early
	if short {
		return 0
	}

	return 0
}

func (v *CSIVolumeStatusCommand) formatBasic(vol *api.CSIVolume) string {
	output := []string{
		fmt.Sprintf("ID|%s", vol.ID),
		// fmt.Sprintf("Name|%s", vol.Name),
		// fmt.Sprintf("External ID|%s", vol.ExternalID),

		fmt.Sprintf("Healthy|%t", vol.Healthy),
		fmt.Sprintf("Controllers Healthy|%d", vol.ControllersHealthy),
		fmt.Sprintf("Controllers Expected|%d", vol.ControllersExpected),
		fmt.Sprintf("Nodes Healthy|%d", vol.NodesHealthy),
		fmt.Sprintf("Nodes Expected|%d", vol.NodesExpected),

		fmt.Sprintf("Access Mode|%s", vol.AccessMode),
		fmt.Sprintf("Attachment Mode|%s", vol.AttachmentMode),
		fmt.Sprintf("Namespace|%s", vol.Namespace),
	}

	return strings.Join(output, "\n")
}

func (v *CSIVolumeStatusCommand) formatTopologies(vol *api.CSIVolume) string {
	var out []string

	// Find the union of all the keys
	head := map[string]string{}
	for _, t := range vol.Topologies {
		for key := range t.Segments {
			if _, ok := head[key]; !ok {
				head[key] = ""
			}
		}
	}

	// Append the header
	var line []string
	for key := range head {
		line = append(line, key)
	}
	out = append(out, strings.Join(line, " "))

	// Append each topology
	for _, t := range vol.Topologies {
		line = []string{}
		for key := range head {
			line = append(line, t.Segments[key])
		}
		out = append(out, strings.Join(line, " "))
	}

	return strings.Join(out, "\n")
}

package command

import (
	"fmt"
	"strings"
)

type AllocSignalCommand struct {
	Meta
}

func (a *AllocSignalCommand) Help() string {
	helpText := `
Usage: nomad alloc signal [options] <allocation> <task>

  signal an existing allocation. This command is used to signal a specific alloc
  and its subtasks. If no task is provided then all of the allocations subtasks
  will recieve the signal.

General Options:

  ` + generalOptionsUsage() + `

Signal Specific Options:

  -verbose
    Show full information.
`
	return strings.TrimSpace(helpText)
}

func (c *AllocSignalCommand) Name() string { return "alloc signal" }

func (c *AllocSignalCommand) Run(args []string) int {
	var verbose bool

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&verbose, "verbose", false, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got exactly one alloc
	args = flags.Args()
	if len(args) != 1 {
		c.Ui.Error("This command takes one argument: <alloc-id>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	allocID := args[0]

	// Truncate the id unless full length is requested
	length := shortId
	if verbose {
		length = fullId
	}

	// Query the allocation info
	if len(allocID) == 1 {
		c.Ui.Error(fmt.Sprintf("Alloc ID must contain at least two characters."))
		return 1
	}

	allocID = sanitizeUUIDPrefix(allocID)

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	allocs, _, err := client.Allocations().PrefixList(allocID)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying allocation: %v", err))
		return 1
	}

	if len(allocs) == 0 {
		c.Ui.Error(fmt.Sprintf("No allocation(s) with prefix or id %q found", allocID))
		return 1
	}

	if len(allocs) > 1 {
		// Format the allocs
		out := formatAllocListStubs(allocs, verbose, length)
		c.Ui.Error(fmt.Sprintf("Prefix matched multiple allocations\n\n%s", out))
		return 1
	}

	// Prefix lookup matched a single allocation
	alloc, _, err := client.Allocations().Info(allocs[0].ID, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying allocation: %s", err))
		return 1
	}

	// Placeholder - List root dir until we implement RPCs for stopping.

	files, _, err := client.AllocFS().List(alloc, "/", nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error listing alloc dir: %s", err))
		return 1
	}

	// Display the file information in a tabular format
	out := make([]string, len(files)+1)
	out[0] = "Mode|Size|Modified Time|Name"
	for i, file := range files {
		fn := file.Name
		if file.IsDir {
			fn = fmt.Sprintf("%s/", fn)
		}
		var size string
		size = fmt.Sprintf("%d", file.Size)
		out[i+1] = fmt.Sprintf("%s|%s|%s|%s",
			file.FileMode,
			size,
			formatTime(file.ModTime),
			fn,
		)
	}
	c.Ui.Output(formatList(out))

	return 0
}

func (a *AllocSignalCommand) Synopsis() string {
	return "Signal a running allocation"
}

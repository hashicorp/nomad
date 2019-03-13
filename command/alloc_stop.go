package command

import (
	"fmt"
	"strings"
)

type AllocStopCommand struct {
	Meta
}

func (a *AllocStopCommand) Help() string {
	helpText := `
Usage: nomad alloc stop [options] <allocation>
Alias: nomad stop

  stop an existing allocation. This command is used to signal a specific alloc
  to shut down. When the allocation has been shut down, it will then be
  rescheduled. An interactive monitoring session will display log lines as the
  allocation completes shutting down. It is safe to exit the monitor early with
  ctrl-c.

General Options:

  ` + generalOptionsUsage() + `

Stop Specific Options:

  -kill-timeout <duration>
    The period of time that the allocation should be allowed to perform a
    graceful shutdown (default 5s). After this time, the process will recieve a
    SIGKILL and be forcefully terminated. If set to '0s', then the allocation
    will be immediately terminated.

  -detach
    Return immediately instead of entering monitor mode. After the
    stop command is submitted, a new evaluation ID is printed to the
    screen, which can be used to examine the rescheduling evaluation using the
    eval-status command.

  -verbose
    Show full information.
`
	return strings.TrimSpace(helpText)
}

func (c *AllocStopCommand) Name() string { return "alloc stop" }

func (c *AllocStopCommand) Run(args []string) int {
	var detach, verbose bool

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&detach, "detach", false, "")
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

func (a *AllocStopCommand) Synopsis() string {
	return "Stop and reschedule a running allocation"
}

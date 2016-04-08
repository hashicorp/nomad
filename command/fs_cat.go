package command

import (
	"fmt"
	"io"
	"os"
	"strings"
)

type FSCatCommand struct {
	Meta
}

func (f *FSCatCommand) Help() string {
	helpText := `
	Usage: nomad fs cat <alloc-id> <path>

	Dispays a file in an allocation directory at the given path.
	The path is relative to the allocation directory and defaults to root if unspecified.

	General Options:

  ` + generalOptionsUsage() + `

Cat Options:

  -verbose
    Show full information.

  -job <job-id>
    Use a random allocation from a specified job-id.
`
	return strings.TrimSpace(helpText)
}

func (f *FSCatCommand) Synopsis() string {
	return "Cat a file in an allocation directory"
}

func (f *FSCatCommand) Run(args []string) int {
	var verbose, job bool
	flags := f.Meta.FlagSet("fs-list", FlagSetClient)
	flags.Usage = func() { f.Ui.Output(f.Help()) }
	flags.BoolVar(&verbose, "verbose", false, "")
	flags.BoolVar(&job, "job", false, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}
	args = flags.Args()

	if len(args) < 1 {
		f.Ui.Error("allocation id is a required parameter")
		return 1
	}

	path := "/"
	if len(args) == 2 {
		path = args[1]
	}

	client, err := f.Meta.Client()
	if err != nil {
		f.Ui.Error(fmt.Sprintf("Error initializing client: %v", err))
		return 1
	}

	// If -job is specified, use random allocation, otherwise use provided allocation
	allocID := args[0]
	if job {
		allocID, err = getRandomJobAlloc(client, args[0])
		if err != nil {
			f.Ui.Error(fmt.Sprintf("Error querying API: %v", err))
			return 1
		}
	}

	// Truncate the id unless full length is requested
	length := shortId
	if verbose {
		length = fullId
	}
	// Query the allocation info
	if len(allocID) == 1 {
		f.Ui.Error(fmt.Sprintf("Alloc ID must contain at least two characters."))
		return 1
	}
	if len(allocID)%2 == 1 {
		// Identifiers must be of even length, so we strip off the last byte
		// to provide a consistent user experience.
		allocID = allocID[:len(allocID)-1]
	}

	allocs, _, err := client.Allocations().PrefixList(allocID)
	if err != nil {
		f.Ui.Error(fmt.Sprintf("Error querying allocation: %v", err))
		return 1
	}
	if len(allocs) == 0 {
		f.Ui.Error(fmt.Sprintf("No allocation(s) with prefix or id %q found", allocID))
		return 1
	}
	if len(allocs) > 1 {
		// Format the allocs
		out := make([]string, len(allocs)+1)
		out[0] = "ID|Eval ID|Job ID|Task Group|Desired Status|Client Status"
		for i, alloc := range allocs {
			out[i+1] = fmt.Sprintf("%s|%s|%s|%s|%s|%s",
				limit(alloc.ID, length),
				limit(alloc.EvalID, length),
				alloc.JobID,
				alloc.TaskGroup,
				alloc.DesiredStatus,
				alloc.ClientStatus,
			)
		}
		f.Ui.Output(fmt.Sprintf("Prefix matched multiple allocations\n\n%s", formatList(out)))
		return 0
	}
	// Prefix lookup matched a single allocation
	alloc, _, err := client.Allocations().Info(allocs[0].ID, nil)
	if err != nil {
		f.Ui.Error(fmt.Sprintf("Error querying allocation: %s", err))
		return 1
	}

	if alloc.DesiredStatus == "failed" {
		allocID := limit(alloc.ID, length)
		msg := fmt.Sprintf(`The allocation %q failed to be placed. To see the cause, run: 
nomad alloc-status %s`, allocID, allocID)
		f.Ui.Error(msg)
		return 0
	}

	// Get the contents of the file
	r, _, err := client.AllocFS().Cat(alloc, path, nil)
	if err != nil {
		f.Ui.Error(fmt.Sprintf("Error reading file: %v", err))
		return 1
	}
	io.Copy(os.Stdout, r)
	return 0
}

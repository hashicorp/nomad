package command

import (
	"fmt"
	"strings"
)

type FSListCommand struct {
	Meta
}

func (f *FSListCommand) Help() string {
	helpText := `
Usage: nomad fs-list alloc-id path

	List displays the contents of the allocation directory for the passed allocation. The path 
	is relative to the root of the alloc dir and defaults to root if unspecified.
`
	return strings.TrimSpace(helpText)
}

func (f *FSListCommand) Synopsis() string {
	return "Lists list of files of an allocation directory"
}

func (f *FSListCommand) Run(args []string) int {
	flags := f.Meta.FlagSet("fs-list", FlagSetClient)
	flags.Usage = func() { f.Ui.Output(f.Help()) }

	if err := flags.Parse(args); err != nil {
		return 1
	}
	args = flags.Args()

	if len(args) < 1 {
		f.Ui.Error("allocation id is a required parameter")
		return 1
	}

	allocID := args[0]
	path := "/"
	if len(args) == 2 {
		path = args[1]
	}

	client, err := f.Meta.Client()
	if err != nil {
		f.Ui.Error(fmt.Sprintf("Error inititalizing client: %v", err))
		return 1
	}

	// Query the allocation info
	alloc, _, err := client.Allocations().Info(allocID, nil)
	if err != nil {
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
					alloc.ID,
					alloc.EvalID,
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
		alloc, _, err = client.Allocations().Info(allocs[0].ID, nil)
		if err != nil {
			f.Ui.Error(fmt.Sprintf("Error querying allocation: %s", err))
			return 1
		}
	}

	// Get the file at the given path
	files, _, err := client.AllocFS().List(alloc, path, nil)
	if err != nil {
		f.Ui.Error(fmt.Sprintf("Error listing alloc dir: %v", err))
		return 1
	}

	// Display the file information in a tabular format
	out := make([]string, len(files)+1)
	out[0] = "Name|Size"
	for i, file := range files {
		fn := file.Name
		if file.IsDir {
			fn = fmt.Sprintf("%s/", fn)
		}
		out[i+1] = fmt.Sprintf("%s|%d",
			fn,
			file.Size)
	}

	f.Ui.Output(formatList(out))
	return 0
}

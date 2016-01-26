package command

import (
	"fmt"
	"strings"
)

type FSStatCommand struct {
	Meta
}

func (f *FSStatCommand) Help() string {
	helpText := `
Usage: nomad fs-stat [alloc-id] [path]
	
	Displays information about a file in an allocation directory at the given path.
	The path is relative to the allocation directory.
`
	return strings.TrimSpace(helpText)
}

func (f *FSStatCommand) Synopsis() string {
	return "Stats a file in an allocation directory"
}

func (f *FSStatCommand) Run(args []string) int {
	flags := f.Meta.FlagSet("fs-list", FlagSetClient)
	flags.Usage = func() { f.Ui.Output(f.Help()) }

	if err := flags.Parse(args); err != nil {
		return 1
	}
	args = flags.Args()

	if len(args) < 1 {
		f.Ui.Error("a valid alloc id is essential")
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

	alloc, _, err := client.Allocations().Info(allocID, nil)
	if err != nil {
		f.Ui.Error(fmt.Sprintf("Error getting alloc: %v", err))
		return 1
	}

	file, _, err := client.AllocFS().Stat(alloc, path, nil)
	if err != nil {
		f.Ui.Error(fmt.Sprintf("Error stating file: %v:", err))
		return 1
	}

	out := make([]string, 2)
	out[0] = "Name|Size"
	if file != nil {
		fn := file.Name
		if file.IsDir {
			fn = fmt.Sprintf("%s/", fn)
		}
		out[1] = fmt.Sprintf("%s|%d", fn, file.Size)
	}
	f.Ui.Output(formatList(out))
	return 0
}

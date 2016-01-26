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
Usage: nomad fs-list [alloc-id] [path]

	Displays the files in the alloc-dir of the given alloc id. The path 
	is relative to the root of the alloc dir.
`
	return strings.TrimSpace(helpText)
}

func (f *FSListCommand) Synopsis() string {
	return "Displays list of files of an alloc dir"
}

func (c *FSListCommand) Run(args []string) int {

	flags := c.Meta.FlagSet("fs-list", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }

	if err := flags.Parse(args); err != nil {
		return 1
	}
	args = flags.Args()

	if len(args) < 1 {
		c.Ui.Error("a valid alloc id is essential")
		return 1
	}

	allocID := args[0]
	path := "/"
	if len(args) == 2 {
		path = args[1]
	}

	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error inititalizing client: %v", err))
		return 1
	}

	alloc, _, err := client.Allocations().Info(allocID, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error getting alloc: %v", err))
		return 1
	}

	files, _, err := client.AllocFS().List(alloc, path, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error listing alloc dir: %v", err))
		return 1
	}

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

	c.Ui.Output(formatList(out))
	return 0
}

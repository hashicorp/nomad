package command

import (
	"fmt"
	"io"
	"os"
	"strings"
)

type LogsCommand struct {
	Meta
}

func (l *LogsCommand) Help() string {
	helpText := `
Usage: nomad logs [options]

Display logs of a task of an allocation which isn't destroyed on a client
	`
	return strings.TrimSpace(helpText)
}

func (l *LogsCommand) Synopsis() string {
	return "Display logs of a task"
}

func (l *LogsCommand) Run(args []string) int {
	var allocID string
	var task string
	var stdout bool
	var stderr bool
	var follow bool
	var lines int

	flags := l.Meta.FlagSet("logs", FlagSetClient)
	flags.StringVar(&allocID, "alloc", "", "allocation id")
	flags.StringVar(&task, "task", "", "task name")
	flags.BoolVar(&stdout, "stdout", false, "stdout buffer")
	flags.BoolVar(&stderr, "stderr", false, "stderr buffer")
	flags.BoolVar(&follow, "follow", follow, "follow")
	flags.IntVar(&lines, "lines", -1, "number of lines")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	if task == "" || allocID == "" {
		l.Ui.Error("Provide a valid task name and alloc id")
		return 1
	}

	if !(stdout || stderr) {
		l.Ui.Error("stderr, stdout or both has to be provided for streaming")
		return 1
	}

	client, err := l.Client()
	if err != nil {
		l.Ui.Error(fmt.Sprintf("error fetching logs: %v", err))
		return 1
	}
	// Query the allocation info
	alloc, _, err := client.Allocations().Info(allocID, nil)
	if err != nil {
		allocs, _, err := client.Allocations().PrefixList(allocID)
		if err != nil {
			l.Ui.Error(fmt.Sprintf("Error querying allocation: %v", err))
			return 1
		}
		if len(allocs) == 0 {
			l.Ui.Error(fmt.Sprintf("No allocation(s) with prefix or id %q found", allocID))
			return 1
		}
		if len(allocs) > 1 {
			// Format the allocs
			out := make([]string, len(allocs)+1)
			out[0] = "ID"
			for i, alloc := range allocs {
				out[i+1] = fmt.Sprintf("%s", alloc.ID)
			}
			l.Ui.Output(fmt.Sprintf("Prefix matched multiple allocations\n\n%s", formatList(out)))
			return 0
		}
		// Prefix lookup matched a single allocation
		alloc, _, err = client.Allocations().Info(allocs[0].ID, nil)
		if err != nil {
			l.Ui.Error(fmt.Sprintf("Error querying allocation: %s", err))
			return 1
		}
	}
	if alloc.ClientStatus == "pending" {
		l.Ui.Error(fmt.Sprintf("task %q hasn't started on the allocation %q", task, alloc))
		return 1
	}

	rdr, err := client.TaskLogs().Get(alloc, task, stdout, stderr, follow, lines)
	if err != nil {
		l.Ui.Error(fmt.Sprintf("error fetching logs: %v", err))
		return 1
	}
	io.Copy(os.Stdout, rdr)

	return 0
}

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
	var alloc string
	var task string
	var stdout bool
	var stderr bool
	var follow bool
	var lines int64

	flags := l.Meta.FlagSet("logs", FlagSetClient)
	flags.StringVar(&alloc, "alloc", "", "allocation id")
	flags.StringVar(&task, "task", "", "task name")
	flags.BoolVar(&stdout, "stdout", true, "stdout buffer")
	flags.BoolVar(&stderr, "stderr", true, "stderr buffer")
	flags.BoolVar(&follow, "follow", follow, "follow")
	flags.Int64Var(&lines, "lines", -1, "number of lines")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	if task == "" || alloc == "" {
		l.Ui.Error("Provide a valid task name and alloc id")
		return 1
	}

	client, err := l.Client()
	if err != nil {
		l.Ui.Error(fmt.Sprintf("error fetching logs: %v", err))
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

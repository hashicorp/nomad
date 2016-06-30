// +build !windows

package command

import (
	"strings"
)

type ShutdownTaskCommand struct {
	Meta
}

func (e *ShutdownTaskCommand) Help() string {
	helpText := `
	This is a command used by Nomad internally to gracefully stop windows executor task."
	`
	return strings.TrimSpace(helpText)
}

func (e *ShutdownTaskCommand) Synopsis() string {
	return "internal - shutdown windows task"
}

func (e *ShutdownTaskCommand) Run(args []string) int {

	return 0
}

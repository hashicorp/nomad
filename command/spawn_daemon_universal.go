// +build !linux

package command

import "errors"

func (c *SpawnDaemonCommand) Run(args []string) int {
	return c.outputStartStatus(errors.New("spawn-daemon not supported"), 1)
}

package command

import (
	"encoding/json"
	"os"
	"strings"
)

type SpawnDaemonCommand struct {
	Meta
}

// Status of executing the user's command.
type SpawnStartStatus struct {
	// ErrorMsg will be empty if the user command was started successfully.
	// Otherwise it will have an error message.
	ErrorMsg string
}

func (c *SpawnDaemonCommand) Help() string {
	helpText := `
Usage: nomad spawn-daemon [options] <daemon_config>

  INTERNAL ONLY

  Spawns a daemon process optionally inside a cgroup. The required daemon_config is a json
  encoding of the DaemonConfig struct containing the isolation configuration and command to run.
  SpawnStartStatus is json serialized to Stdout upon running the user command or if any error
  prevents its execution. If there is no error, the process waits on the users
  command and then json serializes  SpawnExitStatus to Stdout after its termination.

General Options:

  ` + generalOptionsUsage()

	return strings.TrimSpace(helpText)
}

func (c *SpawnDaemonCommand) Synopsis() string {
	return "Spawn a daemon command with configurable isolation."
}

// outputStartStatus is a helper function that outputs a SpawnStartStatus to
// Stdout with the passed error, which may be nil to indicate no error. It
// returns the passed status.
func (c *SpawnDaemonCommand) outputStartStatus(err error, status int) int {
	startStatus := &SpawnStartStatus{}
	enc := json.NewEncoder(os.Stdout)

	if err != nil {
		startStatus.ErrorMsg = err.Error()
	}

	enc.Encode(startStatus)
	return status
}

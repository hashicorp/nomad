package command

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/nomad/helper/raftutil"
	"github.com/posener/complete"
)

type OperatorRaftLogsCommand struct {
	Meta
}

func (c *OperatorRaftLogsCommand) Help() string {
	helpText := `
Usage: nomad operator raft _logs <path to nomad data dir>

  Display the log entries persisted in data dir in json form.

  This is a low-level debugging tool and not subject to Nomad's usual backward
  compatibility guarantees.

  If ACLs are enabled, this command requires a management token.
`
	return strings.TrimSpace(helpText)
}

func (c *OperatorRaftLogsCommand) AutocompleteFlags() complete.Flags {
	return complete.Flags{}
}

func (c *OperatorRaftLogsCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *OperatorRaftLogsCommand) Synopsis() string {
	return "Display raft log content"
}

func (c *OperatorRaftLogsCommand) Name() string { return "operator raft _info" }

func (c *OperatorRaftLogsCommand) Run(args []string) int {
	if len(args) != 1 {
		c.Ui.Error("This command takes one argument: <path>")
		c.Ui.Error(commandErrorText(c))

		return 1
	}

	raftPath, err := raftutil.FindRaftFile(args[0])
	if err != nil {
		c.Ui.Error(err.Error())
		return 1
	}

	logs, warnings, err := raftutil.LogEntries(raftPath)
	if err != nil {
		c.Ui.Error(err.Error())
		return 1
	}

	for _, warning := range warnings {
		c.Ui.Error(warning.Error())
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(logs); err != nil {
		c.Ui.Error(fmt.Sprintf("failed to encode output: %v", err))
		return 1
	}

	return 0
}

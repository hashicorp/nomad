package command

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/nomad/helper/raftutil"
	"github.com/posener/complete"
)

type OperatorRaftStateCommand struct {
	Meta
}

func (c *OperatorRaftStateCommand) Help() string {
	helpText := `
Usage: nomad operator raft _state <path to nomad data dir>

  Display the server state obtained by replaying raft log entries persisted in data dir in json form.

  This is a low-level debugging tool and not subject to Nomad's usual backward
  compatibility guarantees.

Options:

  -last-index=<last_index>
    Set the last log index to be applied, to drop spurious log entries not
    properly committed. If passed last_index is zero or negative, it's perceived
    as an offset from the last index seen in raft.
`
	return strings.TrimSpace(helpText)
}

func (c *OperatorRaftStateCommand) AutocompleteFlags() complete.Flags {
	return complete.Flags{}
}

func (c *OperatorRaftStateCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *OperatorRaftStateCommand) Synopsis() string {
	return "Display raft server state"
}

func (c *OperatorRaftStateCommand) Name() string { return "operator raft _state" }

func (c *OperatorRaftStateCommand) Run(args []string) int {
	var fLastIdx int64

	flags := c.Meta.FlagSet(c.Name(), 0)
	flags.Usage = func() { fmt.Println(c.Help()) }
	flags.Int64Var(&fLastIdx, "last-index", 0, "")

	if err := flags.Parse(args); err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to parse args: %v", err))
		return 1
	}
	args = flags.Args()

	if len(args) != 1 {
		c.Ui.Error("This command takes one argument: <path>")
		c.Ui.Error(commandErrorText(c))

		return 1
	}

	// Find raft.db folder
	raftPath, err := raftutil.FindRaftDir(args[0])
	if err != nil {
		c.Ui.Error(err.Error())
		return 1
	}

	state, err := raftutil.FSMState(raftPath, fLastIdx)
	if err != nil {
		c.Ui.Error(err.Error())
		return 1
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(state); err != nil {
		c.Ui.Error(fmt.Sprintf("failed to encode output: %v", err))
		return 1
	}

	return 0
}

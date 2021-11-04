package command

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/nomad/helper/raftutil"
	"github.com/posener/complete"
)

type OperatorSnapshotStateCommand struct {
	Meta
}

func (c *OperatorSnapshotStateCommand) Help() string {
	helpText := `
Usage: nomad operator snapshot _state <file>

  Displays a JSON representation of state in the snapshot.

  To inspect the file "backup.snap":

    $ nomad operator snapshot _state backup.snap
`
	return strings.TrimSpace(helpText)
}

func (c *OperatorSnapshotStateCommand) AutocompleteFlags() complete.Flags {
	return complete.Flags{}
}

func (c *OperatorSnapshotStateCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *OperatorSnapshotStateCommand) Synopsis() string {
	return "Displays information about a Nomad snapshot file"
}

func (c *OperatorSnapshotStateCommand) Name() string { return "operator snapshot _state" }

func (c *OperatorSnapshotStateCommand) Run(args []string) int {
	// Check that we either got no filename or exactly one.
	if len(args) != 1 {
		c.Ui.Error("This command takes one argument: <file>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	path := args[0]
	f, err := os.Open(path)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error opening snapshot file: %s", err))
		return 1
	}
	defer f.Close()

	state, meta, err := raftutil.RestoreFromArchive(f)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to read archive file: %s", err))
		return 1
	}

	sm := raftutil.StateAsMap(state)
	sm["SnapshotMeta"] = []interface{}{meta}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(sm); err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to encode output: %v", err))
		return 1
	}

	return 0
}

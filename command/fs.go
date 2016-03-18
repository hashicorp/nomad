package command

import (
	"math/rand"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/mitchellh/cli"
)

type FSCommand struct {
	Meta
}

func (f *FSCommand) Help() string {
	return "This command is accessed by using one of the subcommands below."
}

func (f *FSCommand) Synopsis() string {
	return "Inspect the contents of an allocation directory"
}

func (f *FSCommand) Run(args []string) int {
	return cli.RunResultHelp
}

// Get Random Allocation ID from a known jobID. Prefer to use a running allocation,
// but use a dead allocation if no running allocations are found
func getRandomJobAlloc(client *api.Client, jobID string) (string, error) {
	var runningAllocs []*api.AllocationListStub
	allocs, _, err := client.Jobs().Allocations(jobID, nil)
	for _, v := range allocs {
		if v.ClientStatus == "running" {
			runningAllocs = append(runningAllocs, v)
		}
	}
	// If we don't have any allocations running, use dead allocations
	if len(runningAllocs) < 1 {
		runningAllocs = allocs
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	allocID := runningAllocs[r.Intn(len(runningAllocs))].ID
	return allocID, err
}

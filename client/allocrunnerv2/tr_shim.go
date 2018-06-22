package allocrunnerv2

import (
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunnerv2/config"
	"github.com/hashicorp/nomad/client/allocrunnerv2/state"
	trstate "github.com/hashicorp/nomad/client/allocrunnerv2/taskrunner/state"
)

// allocRunnerShim is used to pass the alloc runner to the task runner, exposing
// private methods as public fields.
type allocRunnerShim struct {
	*allocRunner
}

func (a *allocRunnerShim) State() *state.State {
	return a.state
}

func (a *allocRunnerShim) Config() *config.Config {
	return a.config
}

func (a *allocRunnerShim) GetAllocDir() *allocdir.AllocDir {
	return a.allocDir
}

func (a *allocRunnerShim) StateUpdated(trState *trstate.State) error {
	// XXX implement
	return nil
}

package interfaces

import (
	"github.com/hashicorp/nomad/client/allocrunnerv2/state"
)

// RunnnerHook is a lifecycle hook into the life cycle of an allocation runner.
type RunnerHook interface {
	Name() string
}

type RunnerPrerunHook interface {
	RunnerHook
	Prerun() error
}

type RunnerPostrunHook interface {
	RunnerHook
	Postrun() error
}

type RunnerDestroyHook interface {
	RunnerHook
	Destroy() error
}

type RunnerUpdateHook interface {
	RunnerHook
	Update() error
}

// XXX Not sure yet
type RunnerHookFactory func(target HookTarget) (RunnerHook, error)
type HookTarget interface {
	// State retrieves a copy of the target alloc runners state.
	State() *state.State
}

package interfaces

import (
	"context"

	"github.com/hashicorp/nomad/client/allocrunner/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

// RunnnerHook is a lifecycle hook into the life cycle of an allocation runner.
type RunnerHook interface {
	Name() string
}

type RunnerPrerunHook interface {
	RunnerHook
	Prerun(context.Context) error
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
	Update(*RunnerUpdateRequest) error
}

type RunnerUpdateRequest struct {
	Alloc *structs.Allocation
}

// XXX Not sure yet
type RunnerHookFactory func(target HookTarget) (RunnerHook, error)
type HookTarget interface {
	// State retrieves a copy of the target alloc runners state.
	State() *state.State
}

// ShutdownHook may be implemented by AllocRunner or TaskRunner hooks and will
// be called when the agent process is being shutdown gracefully.
type ShutdownHook interface {
	RunnerHook

	Shutdown()
}

package state

import (
	"sync"

	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
)

type State struct {
	sync.RWMutex
	Task  *structs.TaskState
	Hooks map[string]*HookState

	// VaultToken is the current Vault token for the task
	VaultToken string
}

// Copy should be called with the lock held
func (s *State) Copy() *State {
	// Create a copy
	c := &State{
		Task:       s.Task.Copy(),
		Hooks:      make(map[string]*HookState, len(s.Hooks)),
		VaultToken: s.VaultToken,
	}

	// Copy the hooks
	for h, state := range s.Hooks {
		c.Hooks[h] = state.Copy()
	}

	return c
}

type HookState struct {
	SuccessfulOnce bool
	Data           map[string]string
	LastError      error
}

func (h *HookState) Copy() *HookState {
	c := new(HookState)
	*c = *h
	c.Data = helper.CopyMapStringString(c.Data)
	return c
}

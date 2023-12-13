// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"github.com/hashicorp/nomad/plugins/drivers"
	"golang.org/x/exp/maps"
)

// LocalState is Task state which is persisted for use when restarting Nomad
// agents.
type LocalState struct {
	Hooks map[string]*HookState

	// DriverNetwork is the network information returned by the task
	// driver's Start method
	DriverNetwork *drivers.DriverNetwork

	// TaskHandle is the handle used to reattach to the task during recovery
	TaskHandle *drivers.TaskHandle

	// RunComplete is set to true when the TaskRunner.Run() method finishes.
	// It is used to distinguish between a dead task that could be restarted
	// and one that will never run again.
	RunComplete bool
}

func NewLocalState() *LocalState {
	return &LocalState{
		Hooks: make(map[string]*HookState),
	}
}

// Canonicalize ensures LocalState is in a consistent state by initializing
// Hooks and ensuring no HookState's are nil. Useful for cleaning unmarshalled
// state which may be in an unknown state.
func (s *LocalState) Canonicalize() {
	if s.Hooks == nil {
		// Hooks is nil, create it
		s.Hooks = make(map[string]*HookState)
	} else {
		for k, v := range s.Hooks {
			// Remove invalid nil entries from Hooks map
			if v == nil {
				delete(s.Hooks, k)
			}
		}
	}
}

// Copy LocalState. Returns nil if nil.
func (s *LocalState) Copy() *LocalState {
	if s == nil {
		return nil
	}

	// Create a copy
	c := &LocalState{
		Hooks:         make(map[string]*HookState, len(s.Hooks)),
		DriverNetwork: s.DriverNetwork.Copy(),
		TaskHandle:    s.TaskHandle.Copy(),
		RunComplete:   s.RunComplete,
	}

	// Copy the hook state
	for h, state := range s.Hooks {
		c.Hooks[h] = state.Copy()
	}

	return c
}

type HookState struct {
	// Prestart is true if the hook has run Prestart successfully and does
	// not need to run again
	PrestartDone bool

	// Data allows hooks to persist arbitrary state.
	Data map[string]string

	// Environment variables set by the hook that will continue to be set
	// even if PrestartDone=true.
	Env map[string]string
}

// Copy HookState. Returns nil if its nil.
func (h *HookState) Copy() *HookState {
	if h == nil {
		return nil
	}

	c := new(HookState)
	*c = *h
	c.Data = maps.Clone(h.Data)
	c.Env = maps.Clone(h.Env)
	return c
}

func (h *HookState) Equal(o *HookState) bool {
	if h == nil || o == nil {
		return h == o
	}

	if h.PrestartDone != o.PrestartDone {
		return false
	}

	if !maps.Equal(h.Data, o.Data) {
		return false
	}

	return maps.Equal(h.Env, o.Env)
}

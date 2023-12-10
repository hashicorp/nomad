// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !linux

package proclib

// New creates a Wranglers backed by the DefaultWrangler implementation, which
// does not do anything.
func New(configs *Configs) *Wranglers {
	w := &Wranglers{
		configs: configs,
		m:       make(map[Task]ProcessWrangler),
		create:  doNothing(configs),
	}

	return w
}

func doNothing(*Configs) create {
	return func(Task) ProcessWrangler {
		return new(DefaultWrangler)
	}
}

// A DefaultWrangler has a no-op implementation. In the task drivers
// we trust for cleaning themselves up.
type DefaultWrangler struct{}

func (w *DefaultWrangler) Initialize() error {
	return nil
}

func (w *DefaultWrangler) Kill() error {
	return nil
}

func (w *DefaultWrangler) Cleanup() error {
	return nil
}

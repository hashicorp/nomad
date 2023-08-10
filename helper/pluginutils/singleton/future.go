// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package singleton

import (
	"github.com/hashicorp/nomad/helper/pluginutils/loader"
	"github.com/hashicorp/nomad/helper/uuid"
)

// future is a sharable future for retrieving a plugin instance or any error
// that may have occurred during the creation.
type future struct {
	waitCh chan struct{}
	id     string

	err      error
	instance loader.PluginInstance
}

// newFuture returns a new pull future
func newFuture() *future {
	return &future{
		waitCh: make(chan struct{}),
		id:     uuid.Generate(),
	}
}

func (f *future) equal(o *future) bool {
	if f == nil && o == nil {
		return true
	} else if f != nil && o != nil {
		return f.id == o.id
	} else {
		return false
	}
}

// wait waits till the future has a result
func (f *future) wait() *future {
	<-f.waitCh
	return f
}

// result returns the results of the future and should only ever be called after
// wait returns.
func (f *future) result() (loader.PluginInstance, error) {
	return f.instance, f.err
}

// set is used to set the results and unblock any waiter. This may only be
// called once.
func (f *future) set(instance loader.PluginInstance, err error) {
	f.instance = instance
	f.err = err
	close(f.waitCh)
}

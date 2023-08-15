// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package proclib

import (
	"fmt"
	"sync"
)

// Task records the unique coordinates of a task from the perspective of a Nomad
// client running the task, that is to say (alloc_id, task_name).
type Task struct {
	AllocID string
	Task    string
}

func (task Task) String() string {
	return fmt.Sprintf("%s/%s", task.AllocID[0:8], task.Task)
}

type create func(Task) ProcessWrangler

// Wranglers keeps track of the ProcessWrangler created for each task. Some
// operating systems may implement ProcessWranglers to ensure that all of the
// processes created by a Task are killed, going a step beyond trusting the
// task drivers to properly clean things up. (Well, on Linux anyway.)
//
// This state must be restored on Client agent startup.
type Wranglers struct {
	configs *Configs
	create  create

	lock sync.Mutex
	m    map[Task]ProcessWrangler
}

// Setup any process management technique relevant to the operating system and
// its particular configuration.
func (w *Wranglers) Setup(task Task) error {
	w.configs.Logger.Trace("setup client process management", "task", task)

	// create process wrangler for task
	pw := w.create(task)

	// perform any initialization if necessary (e.g. create cgroup)
	// if this doesn't work just keep going; it's up to each task driver
	// implementation to decide if this is a failure mode
	_ = pw.Initialize()

	w.lock.Lock()
	defer w.lock.Unlock()

	// keep track of the process wrangler for task
	w.m[task] = pw

	return nil
}

// Destroy any processes still running that were spawned by task. Ideally the
// task driver should be implemented well enough for this to not be necessary,
// but we protect the Client as best we can regardless.
//
// Note that this is called from a TR.Stop which must be idempotent.
func (w *Wranglers) Destroy(task Task) error {
	w.configs.Logger.Trace("destroy and cleanup remnant task processes", "task", task)

	w.lock.Lock()
	defer w.lock.Unlock()

	if pw, exists := w.m[task]; exists {
		pw.Kill()
		pw.Cleanup()
		delete(w.m, task)
	}

	return nil
}

// A ProcessWrangler "owns" a particular Task on a client, enabling the client
// to kill and cleanup processes created by that Task, without help from the
// task driver. Currently we have implementations only for Linux (via cgroups).
type ProcessWrangler interface {
	Initialize() error
	Kill() error
	Cleanup() error
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"

	"github.com/hashicorp/go-hclog"
	ifs "github.com/hashicorp/nomad/client/allocrunner/interfaces"
	cifs "github.com/hashicorp/nomad/client/interfaces"
	"github.com/hashicorp/nomad/client/lib/proclib"
)

const (
	wranglerHookName = "procisolation"
)

// A wranglerHook provides a mechanism through which the Client can be sure any
// processes spawned by a task forcefully get killed when the task is stopped.
//
// Currently only does anything on Linux with cgroups.
type wranglerHook struct {
	wranglers cifs.ProcessWranglers
	task      proclib.Task
	log       hclog.Logger
}

func newWranglerHook(wranglers cifs.ProcessWranglers, task, allocID string, log hclog.Logger) *wranglerHook {
	return &wranglerHook{
		log:       log.Named(wranglerHookName),
		wranglers: wranglers,
		task: proclib.Task{
			AllocID: allocID,
			Task:    task,
		},
	}
}

func (*wranglerHook) Name() string {
	return wranglerHookName
}

func (wh *wranglerHook) Prestart(_ context.Context, request *ifs.TaskPrestartRequest, _ *ifs.TaskPrestartResponse) error {
	wh.log.Trace("setting up client process management", "task", wh.task)
	return wh.wranglers.Setup(wh.task)
}

func (wh *wranglerHook) Stop(_ context.Context, request *ifs.TaskStopRequest, _ *ifs.TaskStopResponse) error {
	wh.log.Trace("stopping client process mangagement", "task", wh.task)
	return wh.wranglers.Destroy(wh.task)
}

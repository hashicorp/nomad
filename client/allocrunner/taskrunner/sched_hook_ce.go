// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent

package taskrunner

import (
	"fmt"

	"github.com/hashicorp/nomad/nomad/structs"
)

type pauseHook struct{}

func (pauseHook) Name() string { return taskPauseHookName }

func newPauseHook(...any) pauseHook {
	return pauseHook{}
}

type pauseGate struct{}

func newPauseGate(...any) *pauseGate {
	return &pauseGate{}
}

func (*pauseGate) Wait() error {
	return nil
}

func (tr *TaskRunner) SetTaskPauseState(structs.TaskScheduleState) error {
	return fmt.Errorf("Enterprise only")
}

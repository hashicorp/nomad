// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

const (
	// taskPauseHookName is the name of the task pause schedule hook. As an
	// enterprise only feature the implementation is split between
	// sched_hook_ce.go and sched_hook_ent.
	taskPauseHookName = "pause"
)

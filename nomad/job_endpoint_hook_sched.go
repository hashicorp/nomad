// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

// jobSchedHook implements a job Validating admission controller.
//
// The implementation of Validate are in the _ce/_ent files.
type jobSchedHook struct{}

func (jobSchedHook) Name() string {
	return "schedule"
}

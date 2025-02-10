// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

// jobNumaHook implements a job Validating and Mutating admission controller.
//
// The implementations of Validate and Mutate are in _ce/_ent files.
type jobNumaHook struct{}

func (jobNumaHook) Name() string {
	return "numa"
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build windows

package procstats

import (
	"github.com/hashicorp/go-set/v3"
	"github.com/mitchellh/go-ps"
)

// ListByPid will scan the process table and return a set of the process family
// tree starting with executorPID as the root.
func ListByPid(executorPID int) set.Collection[ProcessID] {
	procs := list(executorPID, ps.Processes)
	return procs
}

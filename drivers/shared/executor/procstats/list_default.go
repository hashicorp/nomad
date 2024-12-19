// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !linux && !windows

package procstats

import (
	"context"
	"time"

	"github.com/hashicorp/go-set/v3"
	"github.com/hashicorp/nomad/lib/lang"
	"github.com/shirou/gopsutil/v3/process"
)

// List the process tree starting at the given executorPID
func List(executorPID int) set.Collection[ProcessID] {
	result := set.New[ProcessID](10)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stack := lang.NewStack[int32]()
	stack.Push(int32(executorPID))

	for {
		if stack.Empty() {
			break
		}

		nextPPID := stack.Pop()
		result.Insert(ProcessID(nextPPID))

		p, err := process.NewProcessWithContext(ctx, nextPPID)
		if err != nil {
			continue
		}

		children, err := p.ChildrenWithContext(ctx)
		if err != nil {
			continue
		}

		for _, child := range children {
			stack.Push(child.Pid)
		}
	}

	return result
}

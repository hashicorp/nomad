// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package rawexec

import (
	"testing"

	"github.com/hashicorp/nomad/e2e/v3/cluster3"
	"github.com/hashicorp/nomad/e2e/v3/jobs3"
	"github.com/shoenig/test/must"
)

func TestRawExec(t *testing.T) {
	cluster3.Establish(t,
		cluster3.Leader(),
		cluster3.LinuxClients(1),
	)

	t.Run("testOomAdj", testOomAdj)
	t.Run("testOversubMemory", testOversubMemory)
}

func testOomAdj(t *testing.T) {
	job, cleanup := jobs3.Submit(t, "./input/oomadj.hcl")
	t.Cleanup(cleanup)

	logs := job.TaskLogs("group", "cat")
	must.StrContains(t, logs.Stdout, "0")
}

func testOversubMemory(t *testing.T) {
	job, cleanup := jobs3.Submit(t, "./input/oversub.hcl")
	t.Cleanup(cleanup)

	logs := job.TaskLogs("group", "cat")
	must.StrContains(t, logs.Stdout, "134217728") // 128 mb memory_max
}

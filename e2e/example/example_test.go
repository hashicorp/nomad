// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package example

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/nomad/e2e/v3/cluster3"
	"github.com/hashicorp/nomad/e2e/v3/jobs3"
	"github.com/hashicorp/nomad/e2e/v3/namespaces3"
	"github.com/hashicorp/nomad/e2e/v3/util3"
	"github.com/shoenig/test/must"
)

func TestExample(t *testing.T) {
	cluster3.Establish(t,
		cluster3.Leader(),
		cluster3.LinuxClients(1),
		cluster3.Timeout(3*time.Second),
	)

	t.Run("testSleep", testSleep)
	t.Run("testNamespace", testNamespace)
	t.Run("testEnv", testEnv)
}

func testSleep(t *testing.T) {
	_, cleanup := jobs3.Submit(t, "./input/sleep.hcl")
	t.Cleanup(cleanup)
}

func testNamespace(t *testing.T) {
	name := util3.ShortID("example")

	nsCleanup := namespaces3.Create(t, name)
	t.Cleanup(nsCleanup)

	_, jobCleanup := jobs3.Submit(t, "./input/sleep.hcl", jobs3.Namespace(name))
	t.Cleanup(jobCleanup)
}

func testEnv(t *testing.T) {
	job, cleanup := jobs3.Submit(t, "./input/env.hcl", jobs3.WaitComplete("group"))
	t.Cleanup(cleanup)

	expect := fmt.Sprintf("NOMAD_JOB_ID=%s", job.JobID())
	logs := job.TaskLogs("group", "task")
	must.StrContains(t, logs.Stdout, expect)
}

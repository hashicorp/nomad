// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package oversubscription

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/v3/cluster3"
	"github.com/hashicorp/nomad/e2e/v3/jobs3"
	"github.com/shoenig/test/must"
)

var (
	// store the original scheduler configuration
	origConfig *api.SchedulerConfiguration
)

func TestOversubscription(t *testing.T) {
	cluster3.Establish(t,
		cluster3.Leader(),
		cluster3.LinuxClients(1),
	)

	// store the current state of scheduler configuration so we
	// may restore it after the suite is done
	captureSchedulerConfiguration(t)
	t.Cleanup(func() { restoreSchedulerConfiguration(t) })

	// enable memory oversubscription for these tests
	enableMemoryOversubscription(t)

	t.Run("testDocker", testDocker)
	t.Run("testExec", testExec)
}

func testDocker(t *testing.T) {
	job, jobCleanup := jobs3.Submit(t, "./input/docker.hcl")
	t.Cleanup(jobCleanup)

	// job will cat /sys/fs/cgroup/memory.max which should be
	// set to the 30 megabyte memory_max value
	logs := job.TaskLogs("group", "task")
	must.StrContains(t, logs.Stdout, "31457280")
}

func testExec(t *testing.T) {
	job, jobCleanup := jobs3.Submit(t, "./input/exec.hcl")
	t.Cleanup(jobCleanup)

	// wait for poststart
	time.Sleep(10 * time.Second)

	// job will cat /sys/fs/cgroup/nomad.slice/share.slice/<allocid>.sleep.scope/memory.max
	// which should be set to the 30 megabyte memory_max value
	logs := job.TaskLogs("group", "cat")
	must.StrContains(t, logs.Stdout, "31457280")
}

func captureSchedulerConfiguration(t *testing.T) {
	origConfig = getSchedulerConfiguration(t)
}

func restoreSchedulerConfiguration(t *testing.T) {
	operatorAPI := e2eutil.NomadClient(t).Operator()
	_, _, err := operatorAPI.SchedulerSetConfiguration(origConfig, nil)
	must.NoError(t, err)
}

func enableMemoryOversubscription(t *testing.T) {
	schedulerConfig := getSchedulerConfiguration(t)
	schedulerConfig.MemoryOversubscriptionEnabled = true
	operatorAPI := e2eutil.NomadClient(t).Operator()
	_, _, err := operatorAPI.SchedulerCASConfiguration(schedulerConfig, nil)
	must.NoError(t, err)
}

func getSchedulerConfiguration(t *testing.T) *api.SchedulerConfiguration {
	operatorAPI := e2eutil.NomadClient(t).Operator()
	resp, _, err := operatorAPI.SchedulerGetConfiguration(nil)
	must.NoError(t, err)
	return resp.SchedulerConfig
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package oversubscription

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/v3/cluster3"
	"github.com/hashicorp/nomad/e2e/v3/jobs3"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
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
	t.Run("testRawExec", testRawExec)
	t.Run("testRawExecMax", testRawExecMax)
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

	testFunc := func() error {
		// job will cat /sys/fs/cgroup/nomad.slice/share.slice/<allocid>.sleep.scope/memory.max
		// which should be set to the 30 megabyte memory_max value
		expect := "31457280"
		logs := job.TaskLogs("group", "cat")
		if !strings.Contains(logs.Stdout, expect) {
			return fmt.Errorf("expect '%s' in stdout, got: '%s'", expect, logs.Stdout)
		}
		return nil
	}

	// wait for poststart to run, up to 20 seconds
	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(testFunc),
		wait.Timeout(time.Second*20),
		wait.Gap(time.Second*2),
	))
}

func testRawExec(t *testing.T) {
	job, cleanup := jobs3.Submit(t, "./input/rawexec.hcl")
	t.Cleanup(cleanup)

	logs := job.TaskLogs("group", "cat")
	must.StrContains(t, logs.Stdout, "134217728") // 128 mb memory_max
}

func testRawExecMax(t *testing.T) {
	job, cleanup := jobs3.Submit(t, "./input/rawexecmax.hcl")
	t.Cleanup(cleanup)

	// will print memory.low then memory.max
	logs := job.TaskLogs("group", "cat")
	logsRe := regexp.MustCompile(`67108864\s+max`)
	must.RegexMatch(t, logsRe, logs.Stdout)
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

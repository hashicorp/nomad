// Copyright IBM Corp. 2015, 2026
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

	runWithSchedulerConfigLog(t, "testDocker", testDocker)
	runWithSchedulerConfigLog(t, "testExec", testExec)
	runWithSchedulerConfigLog(t, "testRawExec", testRawExec)
	runWithSchedulerConfigLog(t, "testRawExecMax", testRawExecMax)
}

func runWithSchedulerConfigLog(t *testing.T, name string, testFn func(t *testing.T)) {
	t.Run(name, func(t *testing.T) {
		schedulerConfig := getSchedulerConfiguration(t)
		t.Logf("scheduler MemoryOversubscriptionEnabled before %s: %t", name, schedulerConfig.MemoryOversubscriptionEnabled)
		testFn(t)
	})
}

func testDocker(t *testing.T) {
	waitForMemoryOversubscriptionEnabled(t)

	job, jobCleanup := jobs3.Submit(t, "./input/docker.hcl")
	t.Cleanup(jobCleanup)

	testFunc := func() error {
		// job will cat /sys/fs/cgroup/memory.max which should be
		// set to the 30 megabyte memory_max value
		expect := "31457280"
		logs := job.TaskLogs("group", "task")
		if !strings.Contains(logs.Stdout, expect) {
			return fmt.Errorf("expect '%s' in stdout, got: '%s'\n%s", expect, logs.Stdout, oversubscriptionDebugInfo(t, job.JobID()))
		}
		return nil
	}

	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(testFunc),
		wait.Timeout(time.Second*60),
		wait.Gap(time.Second*2),
	))
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

	// wait for poststart to run, up to 60 seconds.
	// this accounts for variability in exec task start time.
	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(testFunc),
		wait.Timeout(time.Second*60),
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
	waitForMemoryOversubscriptionEnabled(t)

	job, cleanup := jobs3.Submit(t, "./input/rawexecmax.hcl")
	t.Cleanup(cleanup)

	testFunc := func() error {
		logs := job.TaskLogs("group", "cat")

		logsRe := regexp.MustCompile(`67108864\s+max`)
		if !logsRe.MatchString(logs.Stdout) {
			return fmt.Errorf("expect '%s' in stdout, got: '%s'\n%s", logsRe.String(), logs.Stdout, oversubscriptionDebugInfo(t, job.JobID()))
		}
		return nil
	}

	// wait for poststart to run, up to 60 seconds.
	// this accounts for variability in exec task start time.
	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(testFunc),
		wait.Timeout(time.Second*60),
		wait.Gap(time.Second*2),
	))
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
	casResp, _, err := operatorAPI.SchedulerCASConfiguration(schedulerConfig, nil)
	must.NoError(t, err)

	if !casResp.Updated {
		current := getSchedulerConfiguration(t)
		must.True(t, current.MemoryOversubscriptionEnabled,
			must.Sprint("SchedulerCASConfiguration was not updated and memory oversubscription remains disabled"))
	}

	waitForMemoryOversubscriptionEnabled(t)
}

func waitForMemoryOversubscriptionEnabled(t *testing.T) {
	testFunc := func() error {
		nodePool := api.NodePoolDefault
		schedulerConfig := getSchedulerConfiguration(t)
		if !schedulerConfig.MemoryOversubscriptionEnabled {
			return fmt.Errorf("memory oversubscription expected enabled but was disabled")
		}

		nomadClient := e2eutil.NomadClient(t)
		pool, _, err := nomadClient.NodePools().Info(nodePool, nil)
		if err != nil {
			return fmt.Errorf("failed reading node pool %q: %w", nodePool, err)
		}

		if pool.SchedulerConfiguration != nil &&
			pool.SchedulerConfiguration.MemoryOversubscriptionEnabled != nil &&
			!*pool.SchedulerConfiguration.MemoryOversubscriptionEnabled {
			return fmt.Errorf("memory oversubscription disabled by node pool %q override", nodePool)
		}

		return nil
	}

	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(testFunc),
		wait.Timeout(time.Second*30),
		wait.Gap(time.Second),
	))
}

func getSchedulerConfiguration(t *testing.T) *api.SchedulerConfiguration {
	operatorAPI := e2eutil.NomadClient(t).Operator()
	resp, _, err := operatorAPI.SchedulerGetConfiguration(nil)
	must.NoError(t, err)
	return resp.SchedulerConfig
}

func oversubscriptionDebugInfo(t *testing.T, jobID string) string {
	global := getSchedulerConfiguration(t)
	return fmt.Sprintf("oversub-debug: job_id=%s global_memory_oversubscription_enabled=%t", jobID, global.MemoryOversubscriptionEnabled)
}

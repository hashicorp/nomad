// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package servicediscovery

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
	"github.com/stretchr/testify/require"
)

func testChecksHappy(t *testing.T) {
	nomadClient := e2eutil.NomadClient(t)

	// Generate our unique job ID which will be used for this test.
	jobID := "nsd-check-happy-" + uuid.Short()
	jobIDs := []string{jobID}

	// Defer a cleanup function to remove the job. This will trigger if the
	// test fails, unless the cancel function is called.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer e2eutil.CleanupJobsAndGCWithContext(t, ctx, &jobIDs)

	// Register the happy checks job.
	allocStubs := e2eutil.RegisterAndWaitForAllocs(t, nomadClient, jobChecksHappy, jobID, "")
	must.Len(t, 1, allocStubs)

	// wait for the alloc to be running
	e2eutil.WaitForAllocRunning(t, nomadClient, allocStubs[0].ID)

	// Get and test the output of 'nomad alloc checks'.
	require.Eventually(t, func() bool {
		output, err := e2eutil.AllocChecks(allocStubs[0].ID)
		if err != nil {
			return false
		}

		// assert the output contains success
		statusRe := regexp.MustCompile(`Status\s+=\s+success`)
		if !statusRe.MatchString(output) {
			return false
		}

		// assert the output contains 200 status code
		statusCodeRe := regexp.MustCompile(`StatusCode\s+=\s+200`)
		if !statusCodeRe.MatchString(output) {
			return false
		}

		// assert output contains nomad's success string
		return strings.Contains(output, `nomad: http ok`)
	}, 5*time.Second, 200*time.Millisecond)
}

func testChecksSad(t *testing.T) {
	nomadClient := e2eutil.NomadClient(t)

	// Generate our unique job ID which will be used for this test.
	jobID := "nsd-check-sad-" + uuid.Short()
	jobIDs := []string{jobID}

	// Defer a cleanup function to remove the job. This will trigger if the
	// test fails, unless the cancel function is called.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer e2eutil.CleanupJobsAndGCWithContext(t, ctx, &jobIDs)

	// Register the sad checks job.
	allocStubs := e2eutil.RegisterAndWaitForAllocs(t, nomadClient, jobChecksSad, jobID, "")
	must.Len(t, 1, allocStubs)

	// wait for the alloc to be running
	e2eutil.WaitForAllocRunning(t, nomadClient, allocStubs[0].ID)

	// Get and test the output of 'nomad alloc checks'.
	require.Eventually(t, func() bool {
		output, err := e2eutil.AllocChecks(allocStubs[0].ID)
		if err != nil {
			return false
		}

		// assert the output contains failure
		statusRe := regexp.MustCompile(`Status\s+=\s+failure`)
		if !statusRe.MatchString(output) {
			return false
		}

		// assert the output contains 501 status code
		statusCodeRe := regexp.MustCompile(`StatusCode\s+=\s+501`)
		if !statusCodeRe.MatchString(output) {
			return false
		}

		// assert output contains error output from python http.server
		return strings.Contains(output, `<p>Error code explanation: HTTPStatus.NOT_IMPLEMENTED - Server does not support this operation.</p>`)
	}, 5*time.Second, 200*time.Millisecond)
}

func testChecksServiceReRegisterAfterCheckRestart(t *testing.T) {
	const (
		jobChecksAfterRestartMain   = "./input/checks_task_restart_main.nomad"
		jobChecksAfterRestartHelper = "./input/checks_task_restart_helper.nomad"
	)

	nomadClient := e2eutil.NomadClient(t)

	idJobMain := "nsd-check-restart-services-" + uuid.Short()
	idJobHelper := "nsd-check-restart-services-helper-" + uuid.Short()
	jobIDs := []string{idJobMain, idJobHelper}

	// Defer a cleanup function to remove the job. This will trigger if the
	// test fails, unless the cancel function is called.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer e2eutil.CleanupJobsAndGCWithContext(t, ctx, &jobIDs)

	// register the main job
	allocStubs := e2eutil.RegisterAndWaitForAllocs(t, nomadClient, jobChecksAfterRestartMain, idJobMain, "")
	must.Len(t, 1, allocStubs)

	// wait for task restart due to failing health check
	must.Wait(t, wait.InitialSuccess(
		wait.BoolFunc(func() bool {
			allocEvents, err := e2eutil.AllocTaskEventsForJob(idJobMain, "")
			if err != nil {
				t.Log("failed to get task events for job", idJobMain, err)
				return false
			}
			for _, events := range allocEvents {
				for _, event := range events {
					if event["Type"] == "Restarting" {
						return true
					}
				}
			}
			return false
		}),
		wait.Timeout(30*time.Second),
		wait.Gap(3*time.Second),
	))

	runHelper := func(command string) {
		vars := []string{"-var", "nodeID=" + allocStubs[0].NodeID, "-var", "cmd=touch", "-var", "delay=3s"}
		err := e2eutil.RegisterWithArgs(idJobHelper, jobChecksAfterRestartHelper, vars...)
		test.NoError(t, err)
	}

	// register helper job, triggering check to start passing
	runHelper("touch")
	defer func() {
		runHelper("rm")
	}()

	// wait for main task to become healthy
	e2eutil.WaitForAllocStatus(t, nomadClient, allocStubs[0].ID, structs.AllocClientStatusRunning)

	// finally assert we have services
	services := nomadClient.Services()
	serviceStubs, _, err := services.Get("nsd-checks-task-restart-test", nil)
	must.NoError(t, err)
	must.Len(t, 1, serviceStubs)
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package operator_scheduler

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const jobBasic = "./input/basic.nomad"

// TestOperatorScheduler runs the Nomad Operator Scheduler suit of tests which
// focus on the behaviour of the /v1/operator/scheduler API.
func TestOperatorScheduler(t *testing.T) {

	// Wait until we have a usable cluster before running the tests.
	nomadClient := e2eutil.NomadClient(t)
	e2eutil.WaitForLeader(t, nomadClient)
	e2eutil.WaitForNodesReady(t, nomadClient, 1)

	// Run our test cases.
	t.Run("TestOperatorScheduler_ConfigPauseEvalBroker", testConfigPauseEvalBroker)
}

// testConfig tests pausing and un-pausing the eval broker and ensures the
// correct behaviour is observed at each stage.
func testConfigPauseEvalBroker(t *testing.T) {

	nomadClient := e2eutil.NomadClient(t)

	// Generate our job ID which will be used for the entire test.
	jobID := "operator-scheduler-config-pause-eval-broker-" + uuid.Generate()[:8]
	jobIDs := []string{jobID}

	// Defer a cleanup function to remove the job. This will trigger if the
	// test fails, unless the cancel function is called.
	ctx, cancel := context.WithCancel(context.Background())
	defer e2eutil.CleanupJobsAndGCWithContext(t, ctx, &jobIDs)

	// Register the job and ensure the alloc reaches the running state before
	// moving safely on.
	allocStubs := e2eutil.RegisterAndWaitForAllocs(t, nomadClient, jobBasic, jobID, "")
	require.Len(t, allocStubs, 1)
	e2eutil.WaitForAllocRunning(t, nomadClient, allocStubs[0].ID)

	// Get the current scheduler config object.
	schedulerConfig, _, err := nomadClient.Operator().SchedulerGetConfiguration(nil)
	require.NoError(t, err)
	require.NotNil(t, schedulerConfig.SchedulerConfig)

	// Set the eval broker to be paused.
	schedulerConfig.SchedulerConfig.PauseEvalBroker = true

	// Write the config back to Nomad.
	schedulerConfigUpdate, _, err := nomadClient.Operator().SchedulerSetConfiguration(
		schedulerConfig.SchedulerConfig, nil)
	require.NoError(t, err)
	require.True(t, schedulerConfigUpdate.Updated)

	// Perform a deregister call. The call will succeed and create an
	// evaluation. Do not use purge, so we can check the job status when
	// dereigster happens.
	evalID, _, err := nomadClient.Jobs().Deregister(jobID, false, nil)
	require.NoError(t, err)
	require.NotEmpty(t, evalID)

	// Evaluation status is set to pending initially, so there isn't a great
	// way to ensure it doesn't transition to another status other than polling
	// for a long enough time to assume it won't change.
	timedFn := func() error {

		// 5 seconds should be more than enough time for an eval to change
		// status unless the broker is disabled.
		timer := time.NewTimer(5 * time.Second)
		defer timer.Stop()

		for {
			select {
			case <-timer.C:
				return nil
			default:
				evalInfo, _, err := nomadClient.Evaluations().Info(evalID, nil)
				if err != nil {
					return err
				}
				if !assert.Equal(t, "pending", evalInfo.Status) {
					return fmt.Errorf(`expected eval status "pending", got %q`, evalInfo.Status)
				}
				time.Sleep(1 * time.Second)
			}
		}
	}
	require.NoError(t, timedFn())

	// Set the eval broker to be un-paused.
	schedulerConfig.SchedulerConfig.PauseEvalBroker = false

	// Write the config back to Nomad.
	schedulerConfigUpdate, _, err = nomadClient.Operator().SchedulerSetConfiguration(
		schedulerConfig.SchedulerConfig, nil)
	require.NoError(t, err)
	require.True(t, schedulerConfigUpdate.Updated)

	// Ensure the job is stopped, then run the garbage collection to clear out
	// all resources.
	e2eutil.WaitForJobStopped(t, nomadClient, jobID)
	_, err = e2eutil.Command("nomad", "system", "gc")
	require.NoError(t, err)

	// If we have reached this far, we do not need to run the cleanup function.
	cancel()
}

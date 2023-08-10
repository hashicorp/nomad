// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"fmt"
	"time"

	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/testutil"
)

type OnUpdateChecksTest struct {
	framework.TC
	jobIDs []string
}

func (tc *OnUpdateChecksTest) BeforeAll(f *framework.F) {
	// Ensure cluster has leader before running tests
	e2eutil.WaitForLeader(f.T(), tc.Nomad())
	// Ensure that we have at least 1 client node in ready state
	e2eutil.WaitForNodesReady(f.T(), tc.Nomad(), 1)
}

func (tc *OnUpdateChecksTest) AfterEach(f *framework.F) {
	nomadClient := tc.Nomad()
	j := nomadClient.Jobs()

	for _, id := range tc.jobIDs {
		j.Deregister(id, true, nil)
	}
	_, err := e2eutil.Command("nomad", "system", "gc")
	f.NoError(err)
}

// TestOnUpdateCheck_IgnoreWarning_IgnoreErrors ensures that deployments
// complete successfully with service checks that warn and error when on_update
// is specified to ignore either.
func (tc *OnUpdateChecksTest) TestOnUpdateCheck_IgnoreWarning_IgnoreErrors(f *framework.F) {
	uuid := uuid.Generate()
	jobID := fmt.Sprintf("on-update-%s", uuid[0:8])
	tc.jobIDs = append(tc.jobIDs, jobID)

	f.NoError(
		e2eutil.Register(jobID, "consul/input/on_update.nomad"),
		"should have registered successfully",
	)

	wc := &e2eutil.WaitConfig{
		Interval: 1 * time.Second,
		Retries:  60,
	}
	f.NoError(
		e2eutil.WaitForLastDeploymentStatus(jobID, "", "successful", wc),
		"deployment should have completed successfully",
	)

	// register update with on_update = ignore
	// this check errors, deployment should still be successful
	f.NoError(
		e2eutil.Register(jobID, "consul/input/on_update_2.nomad"),
		"should have registered successfully",
	)

	f.NoError(
		e2eutil.WaitForLastDeploymentStatus(jobID, "", "successful", wc),
		"deployment should have completed successfully",
	)
}

// TestOnUpdate_CheckRestart ensures that a service check set to ignore
// warnings still follows the check_restart block if the task becomes
// unhealthy after a deployment is successful.  on_update_check_restart has a
// script check that should report as a warning status for the deployment to
// become healthy. The script check then reports unhealthy and the
// check_restart policy should restart the task
func (tc *OnUpdateChecksTest) TestOnUpdate_CheckRestart(f *framework.F) {
	uuid := uuid.Generate()
	jobID := fmt.Sprintf("on-update-restart-%s", uuid[0:8])
	tc.jobIDs = append(tc.jobIDs, jobID)

	f.NoError(
		e2eutil.Register(jobID, "consul/input/on_update_check_restart.nomad"),
		"should have registered successfully",
	)

	wc := &e2eutil.WaitConfig{
		Interval: 1 * time.Second,
		Retries:  60,
	}
	f.NoError(
		e2eutil.WaitForLastDeploymentStatus(jobID, "", "successful", wc),
		"deployment should have completed successfully",
	)

	// Wait for and ensure that allocation restarted
	testutil.WaitForResultRetries(wc.Retries, func() (bool, error) {
		time.Sleep(wc.Interval)
		allocs, err := e2eutil.AllocTaskEventsForJob(jobID, "")
		if err != nil {
			return false, err
		}

		for allocID, allocEvents := range allocs {
			var allocRestarted bool
			var eventTypes []string
			for _, events := range allocEvents {
				eventTypes = append(eventTypes, events["Type"])
				if events["Type"] == "Restart Signaled" {
					allocRestarted = true
				}
			}
			if allocRestarted {
				return true, nil
			}
			return false, fmt.Errorf("alloc %s expected to restart got %v", allocID, eventTypes)
		}

		return true, nil
	}, func(err error) {
		f.NoError(err)
	})
}

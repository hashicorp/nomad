// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package eval_priority

import (
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
)

type EvalPriorityTest struct {
	framework.TC
	jobIDs []string
}

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "EvalPriority",
		CanRunLocal: true,
		Cases: []framework.TestCase{
			new(EvalPriorityTest),
		},
	})
}

func (tc *EvalPriorityTest) BeforeAll(f *framework.F) {
	e2eutil.WaitForLeader(f.T(), tc.Nomad())
	e2eutil.WaitForNodesReady(f.T(), tc.Nomad(), 1)
}

func (tc *EvalPriorityTest) AfterEach(f *framework.F) {
	for _, id := range tc.jobIDs {
		_, _, err := tc.Nomad().Jobs().Deregister(id, true, nil)
		f.NoError(err)
	}
	tc.jobIDs = []string{}

	_, err := e2eutil.Command("nomad", "system", "gc")
	f.NoError(err)
}

// TestEvalPrioritySet performs a test which registers, updates, and
// deregsiters a job setting the eval priority on every call.
func (tc *EvalPriorityTest) TestEvalPrioritySet(f *framework.F) {

	// Generate a jobID and attempt to register the job using the eval
	// priority. In case there is a problem found here and the job registers,
	// we need to ensure it gets cleaned up.
	jobID := "test-eval-priority-" + uuid.Generate()[0:8]
	f.NoError(e2eutil.RegisterWithArgs(jobID, "eval_priority/inputs/thirteen_job_priority.nomad",
		"-eval-priority=80"))
	tc.jobIDs = append(tc.jobIDs, jobID)

	// Wait for the deployment to finish.
	f.NoError(e2eutil.WaitForLastDeploymentStatus(jobID, "default", "successful", nil))

	// Pull the job evaluation list from the API and ensure that this didn't
	// error and contains two evals.
	//
	// Eval 1: the job registration eval.
	// Eval 2: the deployment watcher eval.
	registerEvals, _, err := tc.Nomad().Jobs().Evaluations(jobID, nil)
	f.NoError(err)
	f.Len(registerEvals, 2, "job expected to have two evals")

	// seenEvals tracks the evaluations we have tested for priority quality
	// against our expected value. This allows us to easily perform multiple
	// checks with confidence.
	seenEvals := map[string]bool{}

	// All evaluations should have the priority set to the overridden priority.
	for _, eval := range registerEvals {
		f.Equal(80, eval.Priority)
		seenEvals[eval.ID] = true
	}

	// Update the job image and set an eval priority higher than the job
	// priority.
	f.NoError(e2eutil.RegisterWithArgs(jobID, "eval_priority/inputs/thirteen_job_priority.nomad",
		"-eval-priority=7", "-var", "image=busybox:1.34"))
	f.NoError(e2eutil.WaitForLastDeploymentStatus(jobID, "default", "successful",
		&e2eutil.WaitConfig{Retries: 200}))

	// Pull the latest list of evaluations for the job which will include those
	// as a result of the job update.
	updateEvals, _, err := tc.Nomad().Jobs().Evaluations(jobID, nil)
	f.NoError(err)
	f.NotNil(updateEvals, "expected non-nil evaluation list response")
	f.NotEmpty(updateEvals, "expected non-empty evaluation list response")

	// Iterate the evals, ignoring those we have already seen and check their
	// priority is as expected.
	for _, eval := range updateEvals {
		if ok := seenEvals[eval.ID]; ok {
			continue
		}
		f.Equal(7, eval.Priority)
		seenEvals[eval.ID] = true
	}

	// Deregister the job using an increased priority.
	deregOpts := api.DeregisterOptions{EvalPriority: 100, Purge: true}
	deregEvalID, _, err := tc.Nomad().Jobs().DeregisterOpts(jobID, &deregOpts, nil)
	f.NoError(err)
	f.NotEmpty(deregEvalID, "expected non-empty evaluation ID")

	// Detail the deregistration evaluation and check its priority.
	evalInfo, _, err := tc.Nomad().Evaluations().Info(deregEvalID, nil)
	f.NoError(err)
	f.Equal(100, evalInfo.Priority)

	// If the job was successfully purged, clear the test suite state.
	if err == nil {
		tc.jobIDs = []string{}
	}
}

// TestEvalPriorityNotSet performs a test which registers, updates, and
// deregsiters a job never setting the eval priority.
func (tc *EvalPriorityTest) TestEvalPriorityNotSet(f *framework.F) {

	// Generate a jobID and attempt to register the job using the eval
	// priority. In case there is a problem found here and the job registers,
	// we need to ensure it gets cleaned up.
	jobID := "test-eval-priority-" + uuid.Generate()[0:8]
	f.NoError(e2eutil.Register(jobID, "eval_priority/inputs/thirteen_job_priority.nomad"))
	tc.jobIDs = append(tc.jobIDs, jobID)

	// Wait for the deployment to finish.
	f.NoError(e2eutil.WaitForLastDeploymentStatus(jobID, "default", "successful", nil))

	// Pull the job evaluation list from the API and ensure that this didn't
	// error and contains two evals.
	//
	// Eval 1: the job registration eval.
	// Eval 2: the deployment watcher eval.
	registerEvals, _, err := tc.Nomad().Jobs().Evaluations(jobID, nil)
	f.NoError(err)
	f.Len(registerEvals, 2, "job expected to have two evals")

	// seenEvals tracks the evaluations we have tested for priority quality
	// against our expected value. This allows us to easily perform multiple
	// checks with confidence.
	seenEvals := map[string]bool{}

	// All evaluations should have the priority set to the job priority.
	for _, eval := range registerEvals {
		f.Equal(13, eval.Priority)
		seenEvals[eval.ID] = true
	}

	// Update the job image without setting an eval priority.
	f.NoError(e2eutil.RegisterWithArgs(jobID, "eval_priority/inputs/thirteen_job_priority.nomad",
		"-var", "image=busybox:1.34"))
	f.NoError(e2eutil.WaitForLastDeploymentStatus(jobID, "default", "successful",
		&e2eutil.WaitConfig{Retries: 200}))

	// Pull the latest list of evaluations for the job which will include those
	// as a result of the job update.
	updateEvals, _, err := tc.Nomad().Jobs().Evaluations(jobID, nil)
	f.NoError(err)
	f.NotNil(updateEvals, "expected non-nil evaluation list response")
	f.NotEmpty(updateEvals, "expected non-empty evaluation list response")

	// Iterate the evals, ignoring those we have already seen and check their
	// priority is as expected.
	for _, eval := range updateEvals {
		if ok := seenEvals[eval.ID]; ok {
			continue
		}
		f.Equal(13, eval.Priority)
		seenEvals[eval.ID] = true
	}

	// Deregister the job without setting an eval priority.
	deregOpts := api.DeregisterOptions{Purge: true}
	deregEvalID, _, err := tc.Nomad().Jobs().DeregisterOpts(jobID, &deregOpts, nil)
	f.NoError(err)
	f.NotEmpty(deregEvalID, "expected non-empty evaluation ID")

	// Detail the deregistration evaluation and check its priority.
	evalInfo, _, err := tc.Nomad().Evaluations().Info(deregEvalID, nil)
	f.NoError(err)
	f.Equal(13, evalInfo.Priority)

	// If the job was successfully purged, clear the test suite state.
	if err == nil {
		tc.jobIDs = []string{}
	}
}

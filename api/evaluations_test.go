// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"fmt"
	"sort"
	"testing"

	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
)

func TestEvaluations_List(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	e := c.Evaluations()

	// Listing when nothing exists returns empty
	result, qm, err := e.List(nil)
	must.NoError(t, err)
	must.Eq(t, 0, qm.LastIndex)
	must.SliceEmpty(t, result)

	// Register a job. This will create an evaluation.
	jobs := c.Jobs()
	job := testJob()
	resp, wm, err := jobs.Register(job, nil)
	must.NoError(t, err)
	assertWriteMeta(t, wm)

	// Check the evaluations again
	result, qm, err = e.List(nil)
	must.NoError(t, err)
	assertQueryMeta(t, qm)

	// if the eval fails fast there can be more than 1
	// but they are in order of most recent first, so look at the last one
	must.Positive(t, len(result))
	idx := len(result) - 1
	must.Eq(t, resp.EvalID, result[idx].ID)

	// wait until the 2nd eval shows up before we try paging
	var results []*Evaluation

	f := func() error {
		results, _, err = e.List(nil)
		if err != nil {
			return fmt.Errorf("failed to list evaluations: %w", err)
		}
		if len(results) < 2 {
			return fmt.Errorf("fewer than 2 results, got: %d", len(results))
		}
		return nil
	}
	must.Wait(t, wait.InitialSuccess(wait.ErrorFunc(f)))

	// query first page
	result, qm, err = e.List(&QueryOptions{
		PerPage: int32(1),
	})
	must.NoError(t, err)
	must.Len(t, 1, result)

	// query second page
	result, qm, err = e.List(&QueryOptions{
		PerPage:   int32(1),
		NextToken: qm.NextToken,
	})
	must.NoError(t, err)
	must.Len(t, 1, result)

	// Query evaluations using a filter.
	results, _, err = e.List(&QueryOptions{
		Filter: `TriggeredBy == "job-register"`,
	})
	must.Len(t, 1, result)
}

func TestEvaluations_PrefixList(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	e := c.Evaluations()

	// Listing when nothing exists returns empty
	result, qm, err := e.PrefixList("abcdef")
	must.NoError(t, err)
	must.Eq(t, 0, qm.LastIndex)
	must.SliceEmpty(t, result)

	// Register a job. This will create an evaluation.
	jobs := c.Jobs()
	job := testJob()
	resp, wm, err := jobs.Register(job, nil)
	must.NoError(t, err)
	assertWriteMeta(t, wm)

	// Check the evaluations again
	result, qm, err = e.PrefixList(resp.EvalID[:4])
	must.NoError(t, err)
	assertQueryMeta(t, qm)

	// Check if we have the right list
	must.Len(t, 1, result)
	must.Eq(t, resp.EvalID, result[0].ID)
}

func TestEvaluations_Info(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	e := c.Evaluations()

	// Querying a nonexistent evaluation returns error
	_, _, err := e.Info("8E231CF4-CA48-43FF-B694-5801E69E22FA", nil)
	must.Error(t, err)

	// Register a job. Creates a new evaluation.
	jobs := c.Jobs()
	job := testJob()
	resp, wm, err := jobs.Register(job, nil)
	must.NoError(t, err)
	assertWriteMeta(t, wm)

	// Try looking up by the new eval ID
	result, qm, err := e.Info(resp.EvalID, nil)
	must.NoError(t, err)
	assertQueryMeta(t, qm)

	// Check that we got the right result
	must.NotNil(t, result)
	must.Eq(t, resp.EvalID, result.ID)

	// Register the job again to get a related eval
	resp, wm, err = jobs.Register(job, nil)
	evals, _, err := e.List(nil)
	must.NoError(t, err)

	// Find an eval that should have related evals
	for _, eval := range evals {
		if eval.NextEval != "" || eval.PreviousEval != "" || eval.BlockedEval != "" {
			result, qm, err = e.Info(eval.ID, &QueryOptions{
				Params: map[string]string{
					"related": "true",
				},
			})
			must.NoError(t, err)
			assertQueryMeta(t, qm)
			must.NotNil(t, result.RelatedEvals)
		}
	}
}

func TestEvaluations_Delete(t *testing.T) {
	testutil.Parallel(t)

	testClient, testServer := makeClient(t, nil, nil)
	defer testServer.Stop()

	// Attempting to delete an evaluation when the eval broker is not paused
	// should return an error.
	wm, err := testClient.Evaluations().Delete([]string{"8E231CF4-CA48-43FF-B694-5801E69E22FA"}, nil)
	must.Nil(t, wm)
	must.ErrorContains(t, err, "eval broker is enabled")

	// Pause the eval broker, and try to delete an evaluation that does not
	// exist.
	schedulerConfig, _, err := testClient.Operator().SchedulerGetConfiguration(nil)
	must.NoError(t, err)
	must.NotNil(t, schedulerConfig)

	schedulerConfig.SchedulerConfig.PauseEvalBroker = true
	schedulerConfigUpdated, _, err := testClient.Operator().SchedulerCASConfiguration(schedulerConfig.SchedulerConfig, nil)
	must.NoError(t, err)
	must.True(t, schedulerConfigUpdated.Updated)

	wm, err = testClient.Evaluations().Delete([]string{"8E231CF4-CA48-43FF-B694-5801E69E22FA"}, nil)
	must.ErrorContains(t, err, "eval not found")
}

func TestEvaluations_Allocations(t *testing.T) {
	testutil.Parallel(t)
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	e := c.Evaluations()

	// Returns empty if no allocations
	allocs, qm, err := e.Allocations("8E231CF4-CA48-43FF-B694-5801E69E22FA", nil)
	must.NoError(t, err)
	must.Eq(t, 0, qm.LastIndex)
	must.SliceEmpty(t, allocs)
}

func TestEvaluations_Sort(t *testing.T) {
	testutil.Parallel(t)
	evals := []*Evaluation{
		{CreateIndex: 2},
		{CreateIndex: 1},
		{CreateIndex: 5},
	}
	sort.Sort(EvalIndexSort(evals))

	expect := []*Evaluation{
		{CreateIndex: 5},
		{CreateIndex: 2},
		{CreateIndex: 1},
	}
	must.Eq(t, expect, evals)
}

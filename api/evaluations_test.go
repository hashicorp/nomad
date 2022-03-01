package api

import (
	"sort"
	"testing"

	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/stretchr/testify/require"
)

func TestEvaluations_List(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	e := c.Evaluations()

	// Listing when nothing exists returns empty
	result, qm, err := e.List(nil)
	require.NoError(t, err)
	require.Equal(t, uint64(0), qm.LastIndex, "bad index")
	require.Equal(t, 0, len(result), "expected 0 evaluations")

	// Register a job. This will create an evaluation.
	jobs := c.Jobs()
	job := testJob()
	resp, wm, err := jobs.Register(job, nil)
	require.NoError(t, err)
	assertWriteMeta(t, wm)

	// Check the evaluations again
	result, qm, err = e.List(nil)
	require.NoError(t, err)
	assertQueryMeta(t, qm)

	// if the eval fails fast there can be more than 1
	// but they are in order of most recent first, so look at the last one
	require.Greater(t, len(result), 0, "expected eval (%s), got none", resp.EvalID)
	idx := len(result) - 1
	require.Equal(t, resp.EvalID, result[idx].ID, "expected eval (%s), got: %#v", resp.EvalID, result[idx])

	// wait until the 2nd eval shows up before we try paging
	results := []*Evaluation{}
	testutil.WaitForResult(func() (bool, error) {
		results, _, err = e.List(nil)
		if len(results) < 2 || err != nil {
			return false, err
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %s", err)
	})

	// query first page
	result, qm, err = e.List(&QueryOptions{
		PerPage: int32(1),
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(result), "expected no evals after last one but got %d: %#v", len(result), result)

	// query second page
	result, qm, err = e.List(&QueryOptions{
		PerPage:   int32(1),
		NextToken: qm.NextToken,
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(result), "expected no evals after last one but got %d: %#v", len(result), result)

	// Query evaluations using a filter.
	results, _, err = e.List(&QueryOptions{
		Filter: `TriggeredBy == "job-register"`,
	})
	require.Equal(t, 1, len(result), "expected 1 eval, got %d", len(result))
}

func TestEvaluations_PrefixList(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	e := c.Evaluations()

	// Listing when nothing exists returns empty
	result, qm, err := e.PrefixList("abcdef")
	require.NoError(t, err)
	require.Equal(t, uint64(0), qm.LastIndex, "bad index")
	require.Equal(t, 0, len(result), "expected 0 evaluations")

	// Register a job. This will create an evaluation.
	jobs := c.Jobs()
	job := testJob()
	resp, wm, err := jobs.Register(job, nil)
	require.NoError(t, err)
	assertWriteMeta(t, wm)

	// Check the evaluations again
	result, qm, err = e.PrefixList(resp.EvalID[:4])
	require.NoError(t, err)
	assertQueryMeta(t, qm)

	// Check if we have the right list
	require.Equal(t, 1, len(result))
	require.Equal(t, resp.EvalID, result[0].ID)
}

func TestEvaluations_Info(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	e := c.Evaluations()

	// Querying a nonexistent evaluation returns error
	_, _, err := e.Info("8E231CF4-CA48-43FF-B694-5801E69E22FA", nil)
	require.Error(t, err)

	// Register a job. Creates a new evaluation.
	jobs := c.Jobs()
	job := testJob()
	resp, wm, err := jobs.Register(job, nil)
	require.NoError(t, err)
	assertWriteMeta(t, wm)

	// Try looking up by the new eval ID
	result, qm, err := e.Info(resp.EvalID, nil)
	require.NoError(t, err)
	assertQueryMeta(t, qm)

	// Check that we got the right result
	require.NotNil(t, result)
	require.Equal(t, resp.EvalID, result.ID)
}

func TestEvaluations_Allocations(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	e := c.Evaluations()

	// Returns empty if no allocations
	allocs, qm, err := e.Allocations("8E231CF4-CA48-43FF-B694-5801E69E22FA", nil)
	require.NoError(t, err)
	require.Equal(t, uint64(0), qm.LastIndex, "bad index")
	require.Equal(t, 0, len(allocs), "expected 0 evaluations")
}

func TestEvaluations_Sort(t *testing.T) {
	t.Parallel()
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
	require.Equal(t, expect, evals)
}

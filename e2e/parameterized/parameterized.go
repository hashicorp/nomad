// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package parameterized

import (
	"fmt"

	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

type ParameterizedTest struct {
	framework.TC
	jobIDs []string
}

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "Parameterized",
		CanRunLocal: true,
		Cases: []framework.TestCase{
			new(ParameterizedTest),
		},
	})
}

func (tc *ParameterizedTest) BeforeAll(f *framework.F) {
	e2eutil.WaitForLeader(f.T(), tc.Nomad())
}

func (tc *ParameterizedTest) AfterEach(f *framework.F) {
	nomadClient := tc.Nomad()
	j := nomadClient.Jobs()

	for _, id := range tc.jobIDs {
		j.Deregister(id, true, nil)
	}
	_, err := e2eutil.Command("nomad", "system", "gc")
	f.NoError(err)
}

func (tc *ParameterizedTest) TestParameterizedDispatch_Basic(f *framework.F) {
	t := f.T()

	uuid := uuid.Generate()
	jobID := fmt.Sprintf("dispatch-%s", uuid[0:8])
	tc.jobIDs = append(tc.jobIDs, jobID)

	// register job
	require.NoError(t, e2eutil.Register(jobID, "parameterized/input/simple.nomad"))

	// force dispatch
	dispatched := 4

	for i := 0; i < dispatched; i++ {
		require.NoError(t, e2eutil.Dispatch(jobID, map[string]string{"i": fmt.Sprintf("%v", i)}, ""))
	}

	testutil.WaitForResult(func() (bool, error) {
		children, err := e2eutil.DispatchedJobs(jobID)
		if err != nil {
			return false, err
		}

		dead := 0
		for _, c := range children {
			if c["Status"] != "dead" {
				return false, fmt.Errorf("expected periodic job to be dead")
			}
			dead++
		}

		if dead != dispatched {
			return false, fmt.Errorf("expected %d but found %d children", dispatched, dead)
		}

		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})

	// Assert there are no pending children
	summary, err := e2eutil.ChildrenJobSummary(jobID)
	require.NoError(t, err)
	require.Len(t, summary, 1)
	require.Equal(t, summary[0]["Pending"], "0")
	require.Equal(t, summary[0]["Running"], "0")
	require.Equal(t, summary[0]["Dead"], fmt.Sprintf("%v", dispatched))
}

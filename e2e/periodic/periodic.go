// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package periodic

import (
	"fmt"

	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

type PeriodicTest struct {
	framework.TC
	jobIDs []string
}

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "Periodic",
		CanRunLocal: true,
		Cases: []framework.TestCase{
			new(PeriodicTest),
		},
	})
}

func (tc *PeriodicTest) BeforeAll(f *framework.F) {
	e2eutil.WaitForLeader(f.T(), tc.Nomad())
}

func (tc *PeriodicTest) AfterEach(f *framework.F) {
	nomadClient := tc.Nomad()
	j := nomadClient.Jobs()

	for _, id := range tc.jobIDs {
		j.Deregister(id, true, nil)
	}
	_, err := e2eutil.Command("nomad", "system", "gc")
	f.NoError(err)
}

func (tc *PeriodicTest) TestPeriodicDispatch_Basic(f *framework.F) {
	t := f.T()

	uuid := uuid.Generate()
	jobID := fmt.Sprintf("periodicjob-%s", uuid[0:8])
	tc.jobIDs = append(tc.jobIDs, jobID)

	// register job
	require.NoError(t, e2eutil.Register(jobID, "periodic/input/simple.nomad"))

	// force dispatch
	require.NoError(t, e2eutil.PeriodicForce(jobID))

	testutil.WaitForResult(func() (bool, error) {
		children, err := e2eutil.PreviouslyLaunched(jobID)
		if err != nil {
			return false, err
		}

		for _, c := range children {
			if c["Status"] == "dead" {
				return true, nil
			}
		}
		return false, fmt.Errorf("expected periodic job to be dead")
	}, func(err error) {
		require.NoError(t, err)
	})

	// Assert there are no pending children
	summary, err := e2eutil.ChildrenJobSummary(jobID)
	require.NoError(t, err)
	require.Len(t, summary, 1)
	require.Equal(t, summary[0]["Pending"], "0")
	require.Equal(t, summary[0]["Running"], "0")
	require.Equal(t, summary[0]["Dead"], "1")
}

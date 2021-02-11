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

	// Get the child job ID
	testutil.WaitForResult(func() (bool, error) {
		childID, err := e2eutil.JobInspectTemplate(jobID, `{{with index . 1}}{{printf "%s" .ID}}{{end}}`)
		if err != nil {
			return false, err
		}
		if childID != "" {
			return true, nil
		}
		return false, fmt.Errorf("expected non-empty periodic child jobID for job %s", jobID)
	}, func(err error) {
		require.NoError(t, err)
	})

	testutil.WaitForResult(func() (bool, error) {
		status, err := e2eutil.JobInspectTemplate(jobID, `{{with index . 1}}{{printf "%s" .Status}}{{end}}`)
		require.NoError(t, err)
		require.NotEmpty(t, status)
		if status == "dead" {
			return true, nil
		}
		return false, fmt.Errorf("expected periodic job to be dead, got %s", status)
	}, func(err error) {
		require.NoError(t, err)
	})

	// Assert there are no pending children
	pending, err := e2eutil.JobInspectTemplate(jobID, `{{with index . 0}}{{printf "%d" .JobSummary.Children.Pending}}{{end}}`)
	require.NoError(t, err)
	require.Equal(t, "0", pending)

	// Assert there are no pending children
	dead, err := e2eutil.JobInspectTemplate(jobID, `{{with index . 0}}{{printf "%d" .JobSummary.Children.Dead}}{{end}}`)
	require.NoError(t, err)
	require.Equal(t, "1", dead)
}

package volumes

import (
	"fmt"
	"os"
	"time"

	"github.com/hashicorp/nomad/api"
	e2e "github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/jobspec"
	"github.com/hashicorp/nomad/testutil"
)

const ns = ""

type VolumesTest struct {
	framework.TC
	jobIDs []string
}

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "Volumes",
		CanRunLocal: true,
		Cases: []framework.TestCase{
			new(VolumesTest),
		},
	})
}

func (tc *VolumesTest) BeforeAll(f *framework.F) {
	e2e.WaitForLeader(f.T(), tc.Nomad())
	e2e.WaitForNodesReady(f.T(), tc.Nomad(), 1)
}

func (tc *VolumesTest) AfterEach(f *framework.F) {
	if os.Getenv("NOMAD_TEST_SKIPCLEANUP") == "1" {
		return
	}

	for _, id := range tc.jobIDs {
		err := e2e.StopJob(id, "-purge")
		f.Assert().NoError(err)
	}
	tc.jobIDs = []string{}

	_, err := e2e.Command("nomad", "system", "gc")
	f.Assert().NoError(err)
}

// TestVolumeMounts exercises host volume and Docker volume functionality for
// the exec and docker task driver, particularly around mounting locations
// within the container and how this is exposed to the user.
func (tc *VolumesTest) TestVolumeMounts(f *framework.F) {

	jobID := "test-node-drain-" + uuid.Generate()[0:8]
	f.NoError(e2e.Register(jobID, "volumes/input/volumes.nomad"))
	tc.jobIDs = append(tc.jobIDs, jobID)

	expected := []string{"running"}
	f.NoError(e2e.WaitForAllocStatusExpected(jobID, ns, expected), "job should be running")

	allocs, err := e2e.AllocsForJob(jobID, ns)
	f.NoError(err, "could not get allocs for job")
	allocID := allocs[0]["ID"]
	nodeID := allocs[0]["Node ID"]

	cmdToExec := fmt.Sprintf("cat /tmp/foo/%s", allocID)

	out, err := e2e.AllocExec(allocID, "docker_task", cmdToExec, ns, nil)
	f.NoError(err, "could not exec into task: docker_task")
	f.Equal(allocID+"\n", out, "alloc data is missing from docker_task")

	out, err = e2e.AllocExec(allocID, "exec_task", cmdToExec, ns, nil)
	f.NoError(err, "could not exec into task: exec_task")
	f.Equal(out, allocID+"\n", "alloc data is missing from exec_task")

	_, err = e2e.Command("nomad", "job", "stop", jobID)
	f.NoError(err, "could not stop job")

	// modify the job so that we make sure it's placed back on the same host.
	// we want to be able to verify that the data from the previous alloc is
	// still there
	job, err := jobspec.ParseFile("volumes/input/volumes.nomad")
	f.NoError(err)
	job.ID = &jobID
	job.Constraints = []*api.Constraint{
		{
			LTarget: "${node.unique.id}",
			RTarget: nodeID,
			Operand: "=",
		},
	}
	_, _, err = tc.Nomad().Jobs().Register(job, nil)
	f.NoError(err, "could not register updated job")

	testutil.WaitForResultRetries(5000, func() (bool, error) {
		time.Sleep(time.Millisecond * 100)
		allocs, err = e2e.AllocsForJob(jobID, ns)
		if err != nil {
			return false, err
		}
		if len(allocs) < 2 {
			return false, fmt.Errorf("no new allocation for %v: %v", jobID, allocs)
		}

		return true, nil
	}, func(e error) {
		f.NoError(e, "failed to get new alloc")

	})

	newAllocID := allocs[0]["ID"]

	newCmdToExec := fmt.Sprintf("cat /tmp/foo/%s", newAllocID)

	out, err = e2e.AllocExec(newAllocID, "docker_task", cmdToExec, ns, nil)
	f.NoError(err, "could not exec into task: docker_task")
	f.Equal(out, allocID+"\n", "previous alloc data is missing from docker_task")

	out, err = e2e.AllocExec(newAllocID, "docker_task", newCmdToExec, ns, nil)
	f.NoError(err, "could not exec into task: docker_task")
	f.Equal(out, newAllocID+"\n", "new alloc data is missing from docker_task")

	out, err = e2e.AllocExec(newAllocID, "exec_task", cmdToExec, ns, nil)
	f.NoError(err, "could not exec into task: exec_task")
	f.Equal(out, allocID+"\n", "previous alloc data is missing from exec_task")

	out, err = e2e.AllocExec(newAllocID, "exec_task", newCmdToExec, ns, nil)
	f.NoError(err, "could not exec into task: exec_task")
	f.Equal(out, newAllocID+"\n", "new alloc data is missing from exec_task")
}

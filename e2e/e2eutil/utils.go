package e2eutil

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/jobspec"
	"github.com/hashicorp/nomad/testutil"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
)

// retries is used to control how many times to retry checking if the cluster has a leader yet
const retries = 500

func WaitForLeader(t *testing.T, nomadClient *api.Client) {
	statusAPI := nomadClient.Status()

	testutil.WaitForResultRetries(retries, func() (bool, error) {
		leader, err := statusAPI.Leader()
		return leader != "", err
	}, func(err error) {
		t.Fatalf("failed to find leader: %v", err)
	})
}

func RegisterAndWaitForAllocs(f *framework.F, nomadClient *api.Client, jobFile string, prefix string) (string, []*api.AllocationListStub) {
	// Parse job
	job, err := jobspec.ParseFile(jobFile)
	require := require.New(f.T())
	require.Nil(err)
	uuid := uuid.Generate()
	jobId := helper.StringToPtr(prefix + uuid[0:8])
	job.ID = jobId

	// Register job
	jobs := nomadClient.Jobs()
	resp, _, err := jobs.Register(job, nil)
	require.Nil(err)
	require.NotEmpty(resp.EvalID)

	g := NewGomegaWithT(f.T())

	// Wrap in retry to wait until placement
	g.Eventually(func() []*api.AllocationListStub {
		// Look for allocations
		allocs, _, _ := jobs.Allocations(*job.ID, false, nil)
		return allocs
	}, 10*time.Second, time.Second).ShouldNot(BeEmpty())

	allocs, _, err := jobs.Allocations(*job.ID, false, nil)
	require.Nil(err)
	return *jobId, allocs
}

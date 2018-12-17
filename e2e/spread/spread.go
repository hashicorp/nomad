package spread

import (
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/jobspec"
	"github.com/stretchr/testify/require"

	. "github.com/onsi/gomega"
	"testing"
)

type BasicSpreadStruct struct {
	framework.TC
	jobIds []string
}

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "Spread",
		CanRunLocal: true,
		Cases: []framework.TestCase{
			new(BasicSpreadStruct),
		},
	})
}

func (tc *BasicSpreadStruct) TestEvenSpread(f *framework.F) {
	nomadClient := tc.Nomad()

	// Parse job
	job, err := jobspec.ParseFile("spread/input/spread1.nomad")
	require := require.New(f.T())
	require.Nil(err)
	jobId := uuid.Generate()
	job.ID = helper.StringToPtr("spr" + jobId[0:8])

	tc.jobIds = append(tc.jobIds, jobId)

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
	}, 2*time.Second, time.Second).ShouldNot(BeEmpty())

	jobAllocs := nomadClient.Allocations()

	allocs, _, _ := jobs.Allocations(*job.ID, false, nil)

	dcToAllocs := make(map[string]int)

	// Verify spread score and alloc distribution
	for _, allocStub := range allocs {
		alloc, _, err := jobAllocs.Info(allocStub.ID, nil)
		require.Nil(err)
		require.NotEmpty(alloc.Metrics.ScoreMetaData)

		node, _, err := nomadClient.Nodes().Info(alloc.NodeID, nil)
		require.Nil(err)

		cnt := dcToAllocs[node.Datacenter]
		cnt++
		dcToAllocs[node.Datacenter] = cnt
	}

	expectedDcToAllocs := make(map[string]int)
	expectedDcToAllocs["dc1"] = 3
	expectedDcToAllocs["dc2"] = 3
	require.Equal(expectedDcToAllocs, dcToAllocs)
}

func (tc *BasicSpreadStruct) TestMultipleSpreads(f *framework.F) {
	nomadClient := tc.Nomad()

	// Parse job
	job, err := jobspec.ParseFile("spread/input/spread2.nomad")
	require := require.New(f.T())
	require.Nil(err)
	jobId := uuid.Generate()
	job.ID = helper.StringToPtr("spr" + jobId[0:8])

	tc.jobIds = append(tc.jobIds, jobId)

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
	}, 2*time.Second, time.Second).ShouldNot(BeEmpty())

	jobAllocs := nomadClient.Allocations()

	allocs, _, _ := jobs.Allocations(*job.ID, false, nil)

	dcToAllocs := make(map[string]int)
	rackToAllocs := make(map[string]int)

	// Verify spread score and alloc distribution
	for _, allocStub := range allocs {
		alloc, _, err := jobAllocs.Info(allocStub.ID, nil)
		require.Nil(err)
		require.NotEmpty(alloc.Metrics.ScoreMetaData)

		node, _, err := nomadClient.Nodes().Info(alloc.NodeID, nil)
		require.Nil(err)

		cnt := dcToAllocs[node.Datacenter]
		cnt++
		dcToAllocs[node.Datacenter] = cnt

		rack := node.Meta["rack"]
		if rack != "" {
			cnt := rackToAllocs[rack]
			cnt++
			rackToAllocs[rack] = rackToAllocs[rack] + 1
		}
	}

	expectedDcToAllocs := make(map[string]int)
	expectedDcToAllocs["dc1"] = 5
	expectedDcToAllocs["dc2"] = 5
	require.Equal(expectedDcToAllocs, dcToAllocs)

	/*
		TODO(preetha): known failure that needs investigation
		expectedRackToAllocs := make(map[string]int)
		expectedRackToAllocs["r1"] = 5
		expectedRackToAllocs["r2"] = 5
		require.Equal(expectedRackToAllocs, rackToAllocs)
	*/

}

func (tc *BasicSpreadStruct) AfterEach(f *framework.F) {
	nomadClient := tc.Nomad()
	jobs := nomadClient.Jobs()
	// Stop all jobs in test
	for _, id := range tc.jobIds {
		jobs.Deregister(id, true, nil)
	}
	// Garbage collect
	nomadClient.System().GarbageCollect()
}

func TestCalledFromGoTest(t *testing.T) {
	framework.New().AddSuites(&framework.TestSuite{
		Component: "foo",
		Cases: []framework.TestCase{
			new(BasicSpreadStruct),
		},
	}).Run(t)
}

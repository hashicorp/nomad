package spread

import (
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/helper/uuid"
)

type SpreadTest struct {
	framework.TC
	jobIds []string
}

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "Spread",
		CanRunLocal: true,
		Cases: []framework.TestCase{
			new(SpreadTest),
		},
	})
}

func (tc *SpreadTest) BeforeAll(f *framework.F) {
	// Ensure cluster has leader before running tests
	e2eutil.WaitForLeader(f.T(), tc.Nomad())
	e2eutil.WaitForNodesReady(f.T(), tc.Nomad(), 4)
}

func (tc *SpreadTest) TestEvenSpread(f *framework.F) {
	nomadClient := tc.Nomad()
	uuid := uuid.Generate()
	jobId := "spread" + uuid[0:8]
	tc.jobIds = append(tc.jobIds, jobId)
	allocs := e2eutil.RegisterAndWaitForAllocs(f.T(), nomadClient, "spread/input/even_spread.nomad", jobId)

	jobAllocs := nomadClient.Allocations()
	dcToAllocs := make(map[string]int)
	require := require.New(f.T())
	// Verify spread score and alloc distribution
	for _, allocStub := range allocs {
		alloc, _, err := jobAllocs.Info(allocStub.ID, nil)
		require.Nil(err)
		require.NotEmpty(alloc.Metrics.ScoreMetaData)

		node, _, err := nomadClient.Nodes().Info(alloc.NodeID, nil)
		require.Nil(err)
		dcToAllocs[node.Datacenter]++
	}

	expectedDcToAllocs := make(map[string]int)
	expectedDcToAllocs["dc1"] = 3
	expectedDcToAllocs["dc2"] = 3
	require.Equal(expectedDcToAllocs, dcToAllocs)
}

func (tc *SpreadTest) TestMultipleSpreads(f *framework.F) {
	nomadClient := tc.Nomad()
	uuid := uuid.Generate()
	jobId := "spread" + uuid[0:8]
	tc.jobIds = append(tc.jobIds, jobId)
	allocs := e2eutil.RegisterAndWaitForAllocs(f.T(), nomadClient, "spread/input/multiple_spread.nomad", jobId)

	jobAllocs := nomadClient.Allocations()
	dcToAllocs := make(map[string]int)
	rackToAllocs := make(map[string]int)

	require := require.New(f.T())
	// Verify spread score and alloc distribution
	for _, allocStub := range allocs {
		alloc, _, err := jobAllocs.Info(allocStub.ID, nil)

		require.Nil(err)
		require.NotEmpty(alloc.Metrics.ScoreMetaData)

		node, _, err := nomadClient.Nodes().Info(alloc.NodeID, nil)
		require.Nil(err)
		dcToAllocs[node.Datacenter]++
		rack := node.Meta["rack"]
		if rack != "" {
			rackToAllocs[rack]++
		}
	}

	expectedDcToAllocs := make(map[string]int)
	expectedDcToAllocs["dc1"] = 5
	expectedDcToAllocs["dc2"] = 5
	require.Equal(expectedDcToAllocs, dcToAllocs)

	expectedRackToAllocs := make(map[string]int)
	expectedRackToAllocs["r1"] = 7
	expectedRackToAllocs["r2"] = 3
	require.Equal(expectedRackToAllocs, rackToAllocs)

}

func (tc *SpreadTest) AfterEach(f *framework.F) {
	nomadClient := tc.Nomad()
	jobs := nomadClient.Jobs()
	// Stop all jobs in test
	for _, id := range tc.jobIds {
		jobs.Deregister(id, true, nil)
	}
	// Garbage collect
	nomadClient.System().GarbageCollect()
}

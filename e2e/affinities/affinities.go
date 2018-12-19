package affinities

import (
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/jobspec"
	"github.com/stretchr/testify/require"

	. "github.com/onsi/gomega"
)

type BasicAffinityTest struct {
	framework.TC
	jobIds []string
}

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "Affinity",
		CanRunLocal: true,
		Cases: []framework.TestCase{
			new(BasicAffinityTest),
		},
	})
}

func (tc *BasicAffinityTest) registerAndWaitForAllocs(f *framework.F, jobFile string, prefix string) []*api.AllocationListStub {
	nomadClient := tc.Nomad()
	// Parse job
	job, err := jobspec.ParseFile(jobFile)
	require := require.New(f.T())
	require.Nil(err)
	uuid := uuid.Generate()
	jobId := helper.StringToPtr(prefix + uuid[0:8])
	job.ID = jobId

	tc.jobIds = append(tc.jobIds, *jobId)

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
	return allocs
}

func (tc *BasicAffinityTest) TestSingleAffinities(f *framework.F) {
	allocs := tc.registerAndWaitForAllocs(f, "affinities/input/single_affinity.nomad", "aff")

	nomadClient := tc.Nomad()
	jobAllocs := nomadClient.Allocations()
	require := require.New(f.T())
	// Verify affinity score metadata
	for _, allocStub := range allocs {
		alloc, _, err := jobAllocs.Info(allocStub.ID, nil)
		require.Nil(err)
		require.NotEmpty(alloc.Metrics.ScoreMetaData)
		for _, sm := range alloc.Metrics.ScoreMetaData {
			score, ok := sm.Scores["node-affinity"]
			if ok {
				require.Equal(1.0, score)
			}
		}
	}

}

func (tc *BasicAffinityTest) TestMultipleAffinities(f *framework.F) {
	allocs := tc.registerAndWaitForAllocs(f, "affinities/input/multiple_affinities.nomad", "aff")

	nomadClient := tc.Nomad()
	jobAllocs := nomadClient.Allocations()
	require := require.New(f.T())

	// Verify affinity score metadata
	for _, allocStub := range allocs {
		alloc, _, err := jobAllocs.Info(allocStub.ID, nil)
		require.Nil(err)
		require.NotEmpty(alloc.Metrics.ScoreMetaData)

		node, _, err := nomadClient.Nodes().Info(alloc.NodeID, nil)
		require.Nil(err)

		dcMatch := node.Datacenter == "dc1"
		rackMatch := node.Meta != nil && node.Meta["rack"] == "r1"

		// Figure out expected node affinity score based on whether both affinities match or just one does
		expectedNodeAffinityScore := 0.0
		if dcMatch && rackMatch {
			expectedNodeAffinityScore = 1.0
		} else if dcMatch || rackMatch {
			expectedNodeAffinityScore = 0.5
		}

		nodeScore := 0.0
		// Find the node's score for this alloc
		for _, sm := range alloc.Metrics.ScoreMetaData {
			score, ok := sm.Scores["node-affinity"]
			if ok && sm.NodeID == alloc.NodeID {
				nodeScore = score
			}
		}
		require.Equal(nodeScore, expectedNodeAffinityScore)
	}
}

func (tc *BasicAffinityTest) TestAntiAffinities(f *framework.F) {
	allocs := tc.registerAndWaitForAllocs(f, "affinities/input/anti_affinities.nomad", "aff")

	nomadClient := tc.Nomad()
	jobAllocs := nomadClient.Allocations()
	require := require.New(f.T())

	// Verify affinity score metadata
	for _, allocStub := range allocs {
		alloc, _, err := jobAllocs.Info(allocStub.ID, nil)
		require.Nil(err)
		require.NotEmpty(alloc.Metrics.ScoreMetaData)

		node, _, err := nomadClient.Nodes().Info(alloc.NodeID, nil)
		require.Nil(err)

		dcMatch := node.Datacenter == "dc1"
		rackMatch := node.Meta != nil && node.Meta["rack"] == "r1"

		// Figure out expected node affinity score based on whether both affinities match or just one does
		expectedAntiAffinityScore := 0.0
		if dcMatch && rackMatch {
			expectedAntiAffinityScore = -1.0
		} else if dcMatch || rackMatch {
			expectedAntiAffinityScore = -0.5
		}

		nodeScore := 0.0

		// Find the node's score for this alloc
		for _, sm := range alloc.Metrics.ScoreMetaData {
			score, ok := sm.Scores["node-affinity"]
			if ok && sm.NodeID == alloc.NodeID {
				nodeScore = score
			}
		}
		require.Equal(nodeScore, expectedAntiAffinityScore)
	}
}

func (tc *BasicAffinityTest) AfterEach(f *framework.F) {
	nomadClient := tc.Nomad()
	jobs := nomadClient.Jobs()
	// Stop all jobs in test
	for _, id := range tc.jobIds {
		jobs.Deregister(id, true, nil)
	}
	// Garbage collect
	nomadClient.System().GarbageCollect()
}

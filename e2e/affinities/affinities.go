// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package affinities

import (
	"slices"

	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
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

func (tc *BasicAffinityTest) BeforeAll(f *framework.F) {
	// Ensure cluster has leader before running tests
	e2eutil.WaitForLeader(f.T(), tc.Nomad())
	// Ensure that we have four client nodes in ready state
	e2eutil.WaitForNodesReady(f.T(), tc.Nomad(), 4)
}

func (tc *BasicAffinityTest) TestSingleAffinities(f *framework.F) {
	nomadClient := tc.Nomad()
	uuid := uuid.Generate()
	jobId := "aff" + uuid[0:8]
	tc.jobIds = append(tc.jobIds, jobId)
	allocs := e2eutil.RegisterAndWaitForAllocs(f.T(), nomadClient, "affinities/input/single_affinity.nomad", jobId, "")

	jobAllocs := nomadClient.Allocations()

	// Verify affinity score metadata
	for _, allocStub := range allocs {
		alloc, _, err := jobAllocs.Info(allocStub.ID, nil)
		must.Nil(f.T(), err)
		must.SliceNotEmpty(f.T(), alloc.Metrics.ScoreMetaData)

		pickedNodeNormScore := 0.0
		normScores := []float64{}
		for _, sm := range alloc.Metrics.ScoreMetaData {
			score, ok := sm.Scores["node-affinity"]
			normScores = append(normScores, sm.NormScore)
			if ok {
				// if there's a node-affinity score, check if this node is the node the
				// allocation was placed on
				if sm.NodeID == allocStub.NodeID {
					must.Eq(f.T(), score, 1.0)
					pickedNodeNormScore = sm.NormScore
				}
			}
		}
		// additionally, make sure that this node had the highest normalized score out
		// of all nodes we got
		must.Eq(f.T(), pickedNodeNormScore, slices.Max(normScores))
	}

}

func (tc *BasicAffinityTest) TestMultipleAffinities(f *framework.F) {
	nomadClient := tc.Nomad()
	uuid := uuid.Generate()
	jobId := "multiaff" + uuid[0:8]
	tc.jobIds = append(tc.jobIds, jobId)
	allocs := e2eutil.RegisterAndWaitForAllocs(f.T(), nomadClient, "affinities/input/multiple_affinities.nomad", jobId, "")

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
	nomadClient := tc.Nomad()
	uuid := uuid.Generate()
	jobId := "antiaff" + uuid[0:8]
	tc.jobIds = append(tc.jobIds, jobId)
	allocs := e2eutil.RegisterAndWaitForAllocs(f.T(), nomadClient, "affinities/input/anti_affinities.nomad", jobId, "")

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

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package csi

import (
	"fmt"
	"os"

	e2e "github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/stretchr/testify/require"
)

// CSINodeOnlyPluginEFSTest exercises the AWS EFS plugin, which is an
// example of a plugin that can run in Node-only mode.
type CSINodeOnlyPluginEFSTest struct {
	framework.TC
	uuid         string
	testJobIDs   []string
	volumeIDs    []string
	pluginJobIDs []string
}

const efsPluginID = "aws-efs0"

func (tc *CSINodeOnlyPluginEFSTest) BeforeAll(f *framework.F) {
	t := f.T()

	_, err := os.Stat("csi/input/volume-efs.hcl")
	if err != nil {
		t.Skip("skipping CSI test because EFS volume spec file missing:", err)
	}

	// Ensure cluster has leader and at least two client
	// nodes in a ready state before running tests
	e2e.WaitForLeader(t, tc.Nomad())
	e2e.WaitForNodesReady(t, tc.Nomad(), 2)
}

// TestEFSVolumeClaim launches AWS EFS plugins and registers an EFS volume
// as a Nomad CSI volume. We then deploy a job that writes to the volume,
// and share the volume with another job which should be able to read the
// data written by the first job.
func (tc *CSINodeOnlyPluginEFSTest) TestEFSVolumeClaim(f *framework.F) {
	t := f.T()
	require := require.New(t)
	nomadClient := tc.Nomad()
	tc.uuid = uuid.Generate()[0:8]

	// deploy the node plugins job (no need for a controller for EFS)
	nodesJobID := "aws-efs-plugin-nodes-" + tc.uuid
	f.NoError(e2e.Register(nodesJobID, "csi/input/plugin-aws-efs-nodes.nomad"))
	tc.pluginJobIDs = append(tc.pluginJobIDs, nodesJobID)

	f.NoError(e2e.WaitForAllocStatusComparison(
		func() ([]string, error) { return e2e.AllocStatuses(nodesJobID, ns) },
		func(got []string) bool {
			for _, status := range got {
				if status != "running" {
					return false
				}
			}
			return true
		}, pluginAllocWait,
	), "plugin job should be running")

	f.NoError(waitForPluginStatusMinNodeCount(efsPluginID, 2, pluginWait),
		"aws-efs0 node plugins did not become healthy")

	// register a volume
	volID := "efs-vol0"
	err := volumeRegister(volID, "csi/input/volume-efs.hcl", "register")
	require.NoError(err)
	tc.volumeIDs = append(tc.volumeIDs, volID)

	// deploy a job that writes to the volume
	writeJobID := "write-efs-" + tc.uuid
	tc.testJobIDs = append(tc.testJobIDs, writeJobID) // ensure failed tests clean up
	f.NoError(e2e.Register(writeJobID, "csi/input/use-efs-volume-write.nomad"))
	f.NoError(
		e2e.WaitForAllocStatusExpected(writeJobID, ns, []string{"running"}),
		"job should be running")

	allocs, err := e2e.AllocsForJob(writeJobID, ns)
	f.NoError(err, "could not get allocs for write job")
	f.Len(allocs, 1, "could not get allocs for write job")
	writeAllocID := allocs[0]["ID"]

	// read data from volume and assert the writer wrote a file to it
	expectedPath := "/task/test/" + writeAllocID
	_, err = readFile(nomadClient, writeAllocID, expectedPath)
	require.NoError(err)

	// Shutdown the writer so we can run a reader.
	// although EFS should support multiple readers, the plugin
	// does not.
	err = e2e.StopJob(writeJobID)
	require.NoError(err)

	// wait for the volume unpublish workflow to complete
	require.NoError(waitForVolumeClaimRelease(volID, reapWait),
		"write-efs alloc claim was not released")

	// deploy a job that reads from the volume
	readJobID := "read-efs-" + tc.uuid
	tc.testJobIDs = append(tc.testJobIDs, readJobID) // ensure failed tests clean up
	f.NoError(e2e.Register(readJobID, "csi/input/use-efs-volume-read.nomad"))
	f.NoError(
		e2e.WaitForAllocStatusExpected(readJobID, ns, []string{"running"}),
		"job should be running")

	allocs, err = e2e.AllocsForJob(readJobID, ns)
	f.NoError(err, "could not get allocs for read job")
	f.Len(allocs, 1, "could not get allocs for read job")
	readAllocID := allocs[0]["ID"]

	// read data from volume and assert the writer wrote a file to it
	require.NoError(err)
	_, err = readFile(nomadClient, readAllocID, expectedPath)
	require.NoError(err)
}

func (tc *CSINodeOnlyPluginEFSTest) AfterEach(f *framework.F) {

	// Stop all jobs in test
	for _, id := range tc.testJobIDs {
		err := e2e.StopJob(id, "-purge")
		f.Assert().NoError(err)
	}
	tc.testJobIDs = []string{}

	// Deregister all volumes in test
	for _, id := range tc.volumeIDs {
		// make sure all the test jobs have finished unpublishing claims
		err := waitForVolumeClaimRelease(id, reapWait)
		f.Assert().NoError(err, "volume claims were not released")

		out, err := e2e.Command("nomad", "volume", "deregister", id)
		assertNoErrorElseDump(f, err,
			fmt.Sprintf("could not deregister volume:\n%v", out), tc.pluginJobIDs)
	}
	tc.volumeIDs = []string{}

	// Deregister all plugin jobs in test
	for _, id := range tc.pluginJobIDs {
		err := e2e.StopJob(id, "-purge")
		f.Assert().NoError(err)
	}
	tc.pluginJobIDs = []string{}

	// Garbage collect
	out, err := e2e.Command("nomad", "system", "gc")
	f.Assert().NoError(err, out)
}

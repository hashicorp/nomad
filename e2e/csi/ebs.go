package csi

import (
	"fmt"

	e2e "github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
)

// CSIControllerPluginEBSTest exercises the AWS EBS plugin, which is an
// example of a plugin that supports most of the CSI Controller RPCs.
type CSIControllerPluginEBSTest struct {
	framework.TC
	uuid         string
	testJobIDs   []string
	volumeIDs    []string
	pluginJobIDs []string
}

const ebsPluginID = "aws-ebs0"

// BeforeAll waits for the cluster to be ready, deploys the CSI plugins, and
// creates two EBS volumes for use in the test.
func (tc *CSIControllerPluginEBSTest) BeforeAll(f *framework.F) {
	e2e.WaitForLeader(f.T(), tc.Nomad())
	e2e.WaitForNodesReady(f.T(), tc.Nomad(), 2)

	tc.uuid = uuid.Generate()[0:8]

	// deploy the controller plugin job
	controllerJobID := "aws-ebs-plugin-controller-" + tc.uuid
	f.NoError(e2e.Register(controllerJobID, "csi/input/plugin-aws-ebs-controller.nomad"))
	tc.pluginJobIDs = append(tc.pluginJobIDs, controllerJobID)
	expected := []string{"running", "running"}
	f.NoError(
		e2e.WaitForAllocStatusExpected(controllerJobID, ns, expected),
		"job should be running")

	// deploy the node plugins job
	nodesJobID := "aws-ebs-plugin-nodes-" + tc.uuid
	f.NoError(e2e.Register(nodesJobID, "csi/input/plugin-aws-ebs-nodes.nomad"))
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
		}, nil,
	))

	f.NoError(waitForPluginStatusControllerCount(ebsPluginID, 2, pluginWait),
		"aws-ebs0 controller plugins did not become healthy")
	f.NoError(waitForPluginStatusMinNodeCount(ebsPluginID, 2, pluginWait),
		"aws-ebs0 node plugins did not become healthy")

	// ideally we'd wait until after we check `nomad volume status -verbose`
	// to verify these volumes are ready, but the plugin doesn't support the
	// CSI ListVolumes RPC
	volID := "ebs-vol[0]"
	err := volumeRegister(volID, "csi/input/ebs-volume0.hcl", "create")
	requireNoErrorElseDump(f, err, "could not create volume", tc.pluginJobIDs)
	tc.volumeIDs = append(tc.volumeIDs, volID)

	volID = "ebs-vol[1]"
	err = volumeRegister(volID, "csi/input/ebs-volume1.hcl", "create")
	requireNoErrorElseDump(f, err, "could not create volume", tc.pluginJobIDs)
	tc.volumeIDs = append(tc.volumeIDs, volID)
}

// AfterAll cleans up the volumes and plugin jobs created by the test.
func (tc *CSIControllerPluginEBSTest) AfterAll(f *framework.F) {

	// Stop all jobs in test
	for _, id := range tc.testJobIDs {
		out, err := e2e.Command("nomad", "job", "stop", "-purge", id)
		f.Assert().NoError(err, out)
	}
	tc.testJobIDs = []string{}

	for _, volID := range tc.volumeIDs {
		err := waitForVolumeClaimRelease(volID, reapWait)
		f.Assert().NoError(err, "volume claims were not released")

		out, err := e2e.Command("nomad", "volume", "delete", volID)
		assertNoErrorElseDump(f, err,
			fmt.Sprintf("could not delete volume:\n%v", out), tc.pluginJobIDs)
	}

	// Deregister all plugin jobs in test
	for _, id := range tc.pluginJobIDs {
		out, err := e2e.Command("nomad", "job", "stop", "-purge", id)
		f.Assert().NoError(err, out)
	}
	tc.pluginJobIDs = []string{}

	// Garbage collect
	out, err := e2e.Command("nomad", "system", "gc")
	f.Assert().NoError(err, out)

}

// TestVolumeClaim exercises the volume publish/unpublish workflows for the
// EBS plugin.
func (tc *CSIControllerPluginEBSTest) TestVolumeClaim(f *framework.F) {
	nomadClient := tc.Nomad()

	// deploy a job that writes to the volume
	writeJobID := "write-ebs-" + tc.uuid
	f.NoError(e2e.Register(writeJobID, "csi/input/use-ebs-volume.nomad"))
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
	f.NoError(err)

	// Shutdown (and purge) the writer so we can run a reader.
	// we could mount the EBS volume with multi-attach, but we
	// want this test to exercise the unpublish workflow.
	_, err = e2e.Command("nomad", "job", "stop", "-purge", writeJobID)
	f.NoError(err)

	// wait for the volume unpublish workflow to complete
	for _, volID := range tc.volumeIDs {
		err := waitForVolumeClaimRelease(volID, reapWait)
		f.NoError(err, "volume claims were not released")
	}

	// deploy a job so we can read from the volume
	readJobID := "read-ebs-" + tc.uuid
	tc.testJobIDs = append(tc.testJobIDs, readJobID) // ensure failed tests clean up
	f.NoError(e2e.Register(readJobID, "csi/input/use-ebs-volume.nomad"))
	f.NoError(
		e2e.WaitForAllocStatusExpected(readJobID, ns, []string{"running"}),
		"job should be running")

	allocs, err = e2e.AllocsForJob(readJobID, ns)
	f.NoError(err, "could not get allocs for read job")
	f.Len(allocs, 1, "could not get allocs for read job")
	readAllocID := allocs[0]["ID"]

	// read data from volume and assert we can read the file the writer wrote
	expectedPath = "/task/test/" + readAllocID
	_, err = readFile(nomadClient, readAllocID, expectedPath)
	f.NoError(err)
}

// TestSnapshot exercises the snapshot commands.
func (tc *CSIControllerPluginEBSTest) TestSnapshot(f *framework.F) {

	out, err := e2e.Command("nomad", "volume", "snapshot", "create",
		tc.volumeIDs[0], "snap-"+tc.uuid)
	requireNoErrorElseDump(f, err, "could not create volume snapshot", tc.pluginJobIDs)

	snaps, err := e2e.ParseColumns(out)

	defer func() {
		_, err := e2e.Command("nomad", "volume", "snapshot", "delete",
			ebsPluginID, snaps[0]["Snapshot ID"])
		requireNoErrorElseDump(f, err, "could not delete volume snapshot", tc.pluginJobIDs)
	}()

	f.NoError(err, fmt.Sprintf("could not parse output:\n%v", out))
	f.Len(snaps, 1, fmt.Sprintf("could not parse output:\n%v", out))

	out, err = e2e.Command("nomad", "volume", "snapshot", "list")
	requireNoErrorElseDump(f, err, "could not list volume snapshots", tc.pluginJobIDs)
	f.Contains(out, snaps[0]["ID"],
		fmt.Sprintf("volume snapshot list did not include expected snapshot:\n%v", out))
}

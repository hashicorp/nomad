package csi

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"time"

	"github.com/hashicorp/hcl"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/stretchr/testify/require"
)

type CSIVolumesTest struct {
	framework.TC
	testJobIDs   []string
	volumeIDs    []string
	pluginJobIDs []string
}

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "CSI",
		CanRunLocal: true,
		Consul:      false,
		Cases: []framework.TestCase{
			new(CSIVolumesTest),
		},
	})
}

func (tc *CSIVolumesTest) BeforeAll(f *framework.F) {
	t := f.T()
	// Ensure cluster has leader and at least two client
	// nodes in a ready state before running tests
	e2eutil.WaitForLeader(t, tc.Nomad())
	e2eutil.WaitForNodesReady(t, tc.Nomad(), 2)
}

// TestEBSVolumeClaim launches AWS EBS plugins and registers an EBS volume
// as a Nomad CSI volume. We then deploy a job that writes to the volume,
// stop that job, and reuse the volume for another job which should be able
// to read the data written by the first job.
func (tc *CSIVolumesTest) TestEBSVolumeClaim(f *framework.F) {
	t := f.T()
	require := require.New(t)
	nomadClient := tc.Nomad()
	uuid := uuid.Generate()

	// deploy the controller plugin job
	controllerJobID := "aws-ebs-plugin-controller-" + uuid[0:8]
	tc.pluginJobIDs = append(tc.pluginJobIDs, controllerJobID)
	e2eutil.RegisterAndWaitForAllocs(t, nomadClient,
		"csi/input/plugin-aws-ebs-controller.nomad", controllerJobID, "")

	// deploy the node plugins job
	nodesJobID := "aws-ebs-plugin-nodes-" + uuid[0:8]
	tc.pluginJobIDs = append(tc.pluginJobIDs, nodesJobID)
	e2eutil.RegisterAndWaitForAllocs(t, nomadClient,
		"csi/input/plugin-aws-ebs-nodes.nomad", nodesJobID, "")

	// wait for plugin to become healthy
	require.Eventuallyf(func() bool {
		plugin, _, err := nomadClient.CSIPlugins().Info("aws-ebs0", nil)
		if err != nil {
			return false
		}
		if plugin.ControllersHealthy < 2 || plugin.NodesHealthy < 2 {
			return false
		}
		return true
		// TODO(tgross): cut down this time after fixing
		// https://github.com/hashicorp/nomad/issues/7296
	}, 90*time.Second, 5*time.Second, "aws-ebs0 plugins did not become healthy")

	// register a volume
	volID := "ebs-vol0"
	vol, err := parseVolumeFile("csi/input/volume-ebs.hcl")
	require.NoError(err)
	_, err = nomadClient.CSIVolumes().Register(vol, nil)
	require.NoError(err)
	tc.volumeIDs = append(tc.volumeIDs, volID)

	// deploy a job that writes to the volume
	writeJobID := "write-ebs-" + uuid[0:8]
	tc.testJobIDs = append(tc.testJobIDs, writeJobID)
	writeAllocs := e2eutil.RegisterAndWaitForAllocs(t, nomadClient,
		"csi/input/use-ebs-volume.nomad", writeJobID, "")
	writeAllocID := writeAllocs[0].ID
	tc.testJobIDs = append(tc.testJobIDs, writeJobID) // ensure failed tests clean up
	e2eutil.WaitForAllocRunning(t, nomadClient, writeAllocID)

	// read data from volume and assert the writer wrote a file to it
	writeAlloc, _, err := nomadClient.Allocations().Info(writeAllocID, nil)
	require.NoError(err)
	expectedPath := "/local/test/" + writeAllocID
	_, err = readFile(nomadClient, writeAlloc, expectedPath)
	require.NoError(err)

	// Shutdown (and purge) the writer so we can run a reader.
	// we could mount the EBS volume with multi-attach, but we
	// want this test to exercise the unpublish workflow.
	// this runs the equivalent of 'nomad job stop -purge'
	nomadClient.Jobs().Deregister(writeJobID, true, nil)
	// instead of waiting for the alloc to stop, wait for the volume claim gc run
	require.Eventuallyf(func() bool {
		vol, _, err := nomadClient.CSIVolumes().Info(volID, nil)
		if err != nil {
			return false
		}
		return len(vol.WriteAllocs) == 0
	}, 90*time.Second, 5*time.Second, "write-ebs alloc claim was not released")

	// deploy a job so we can read from the volume
	readJobID := "read-ebs-" + uuid[0:8]
	tc.testJobIDs = append(tc.testJobIDs, readJobID)
	readAllocs := e2eutil.RegisterAndWaitForAllocs(t, nomadClient,
		"csi/input/use-ebs-volume.nomad", readJobID, "")
	readAllocID := readAllocs[0].ID
	e2eutil.WaitForAllocRunning(t, nomadClient, readAllocID)

	// read data from volume and assert the writer wrote a file to it
	readAlloc, _, err := nomadClient.Allocations().Info(readAllocID, nil)
	require.NoError(err)
	_, err = readFile(nomadClient, readAlloc, expectedPath)
	require.NoError(err)
}

// TestEFSVolumeClaim launches AWS EFS plugins and registers an EFS volume
// as a Nomad CSI volume. We then deploy a job that writes to the volume,
// and share the volume with another job which should be able to read the
// data written by the first job.
func (tc *CSIVolumesTest) TestEFSVolumeClaim(f *framework.F) {
	t := f.T()
	require := require.New(t)
	nomadClient := tc.Nomad()
	uuid := uuid.Generate()

	// deploy the node plugins job (no need for a controller for EFS)
	nodesJobID := "aws-efs-plugin-nodes-" + uuid[0:8]
	tc.pluginJobIDs = append(tc.pluginJobIDs, nodesJobID)
	e2eutil.RegisterAndWaitForAllocs(t, nomadClient,
		"csi/input/plugin-aws-efs-nodes.nomad", nodesJobID, "")

	// wait for plugin to become healthy
	require.Eventuallyf(func() bool {
		plugin, _, err := nomadClient.CSIPlugins().Info("aws-efs0", nil)
		if err != nil {
			return false
		}
		if plugin.NodesHealthy < 2 {
			return false
		}
		return true
		// TODO(tgross): cut down this time after fixing
		// https://github.com/hashicorp/nomad/issues/7296
	}, 90*time.Second, 5*time.Second, "aws-efs0 plugins did not become healthy")

	// register a volume
	volID := "efs-vol0"
	vol, err := parseVolumeFile("csi/input/volume-efs.hcl")
	require.NoError(err)
	_, err = nomadClient.CSIVolumes().Register(vol, nil)
	require.NoError(err)
	tc.volumeIDs = append(tc.volumeIDs, volID)

	// deploy a job that writes to the volume
	writeJobID := "write-efs-" + uuid[0:8]
	writeAllocs := e2eutil.RegisterAndWaitForAllocs(t, nomadClient,
		"csi/input/use-efs-volume-write.nomad", writeJobID, "")
	writeAllocID := writeAllocs[0].ID
	tc.testJobIDs = append(tc.testJobIDs, writeJobID) // ensure failed tests clean up
	e2eutil.WaitForAllocRunning(t, nomadClient, writeAllocID)

	// read data from volume and assert the writer wrote a file to it
	writeAlloc, _, err := nomadClient.Allocations().Info(writeAllocID, nil)
	require.NoError(err)
	expectedPath := "/local/test/" + writeAllocID
	_, err = readFile(nomadClient, writeAlloc, expectedPath)
	require.NoError(err)

	// Shutdown the writer so we can run a reader.
	// although EFS should support multiple readers, the plugin
	// does not.
	// this runs the equivalent of 'nomad job stop'
	nomadClient.Jobs().Deregister(writeJobID, false, nil)
	// instead of waiting for the alloc to stop, wait for the volume claim gc run
	require.Eventuallyf(func() bool {
		vol, _, err := nomadClient.CSIVolumes().Info(volID, nil)
		if err != nil {
			return false
		}
		return len(vol.WriteAllocs) == 0
	}, 90*time.Second, 5*time.Second, "write-efs alloc claim was not released")

	// deploy a job that reads from the volume.
	readJobID := "read-efs-" + uuid[0:8]
	tc.testJobIDs = append(tc.testJobIDs, readJobID)
	readAllocs := e2eutil.RegisterAndWaitForAllocs(t, nomadClient,
		"csi/input/use-efs-volume-read.nomad", readJobID, "")
	e2eutil.WaitForAllocRunning(t, nomadClient, readAllocs[0].ID)

	// read data from volume and assert the writer wrote a file to it
	readAlloc, _, err := nomadClient.Allocations().Info(readAllocs[0].ID, nil)
	require.NoError(err)
	_, err = readFile(nomadClient, readAlloc, expectedPath)
	require.NoError(err)
}

func (tc *CSIVolumesTest) AfterEach(f *framework.F) {
	nomadClient := tc.Nomad()
	jobs := nomadClient.Jobs()
	// Stop all jobs in test
	for _, id := range tc.testJobIDs {
		jobs.Deregister(id, true, nil)
	}
	// Deregister all volumes in test
	for _, id := range tc.volumeIDs {
		nomadClient.CSIVolumes().Deregister(id, true, nil)
	}
	// Deregister all plugin jobs in test
	for _, id := range tc.pluginJobIDs {
		jobs.Deregister(id, true, nil)
	}

	// Garbage collect
	nomadClient.System().GarbageCollect()
}

// TODO(tgross): replace this w/ AllocFS().Stat() after
// https://github.com/hashicorp/nomad/issues/7365 is fixed
func readFile(client *api.Client, alloc *api.Allocation, path string) (bytes.Buffer, error) {
	ctx, cancelFn := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelFn()

	var stdout, stderr bytes.Buffer
	_, err := client.Allocations().Exec(ctx,
		alloc, "task", false,
		[]string{"cat", path},
		os.Stdin, &stdout, &stderr,
		make(chan api.TerminalSize), nil)
	return stdout, err
}

// TODO(tgross): this is taken from `nomad volume register` but
// it would be nice if we could expose this with a ParseFile as
// we do for api.Job.
func parseVolumeFile(filepath string) (*api.CSIVolume, error) {

	rawInput, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	ast, err := hcl.Parse(string(rawInput))
	if err != nil {
		return nil, err
	}

	output := &api.CSIVolume{}
	err = hcl.DecodeObject(output, ast)
	if err != nil {
		return nil, err
	}

	// api.CSIVolume doesn't have the type field, it's used only for
	// dispatch in parseVolumeType
	helper.RemoveEqualFold(&output.ExtraKeysHCL, "type")
	err = helper.UnusedKeys(output)
	if err != nil {
		return nil, err
	}

	return output, nil
}

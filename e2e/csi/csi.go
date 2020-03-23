package csi

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/stretchr/testify/require"
)

type CSIVolumesTest struct {
	framework.TC
	jobIds    []string
	volumeIDs *volumeConfig
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

type volumeConfig struct {
	EBSVolumeID string `json:"ebs_volume"`
	EFSVolumeID string `json:"efs_volume"`
}

func (tc *CSIVolumesTest) BeforeAll(f *framework.F) {
	t := f.T()
	// The volume IDs come from the external provider, so we need
	// to read the configuration out of our Terraform output.
	rawjson, err := ioutil.ReadFile("csi/input/volumes.json")
	if err != nil {
		t.Skip("volume ID configuration not found, try running 'terraform output volumes > ../csi/input/volumes.json'")
	}
	volumeIDs := &volumeConfig{}
	err = json.Unmarshal(rawjson, volumeIDs)
	if err != nil {
		t.Fatal("volume ID configuration could not be read")
	}

	tc.volumeIDs = volumeIDs

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
	tc.jobIds = append(tc.jobIds, controllerJobID)
	e2eutil.RegisterAndWaitForAllocs(t, nomadClient,
		"csi/input/plugin-aws-ebs-controller.nomad", controllerJobID, "")

	// deploy the node plugins job
	nodesJobID := "aws-ebs-plugin-nodes-" + uuid[0:8]
	tc.jobIds = append(tc.jobIds, nodesJobID)
	e2eutil.RegisterAndWaitForAllocs(t, nomadClient,
		"csi/input/plugin-aws-ebs-nodes.nomad", nodesJobID, "")

	// wait for plugin to become healthy
	require.Eventually(func() bool {
		plugin, _, err := nomadClient.CSIPlugins().Info("aws-ebs0", nil)
		if err != nil {
			return false
		}
		if plugin.ControllersHealthy != 1 || plugin.NodesHealthy < 2 {
			return false
		}
		return true
		// TODO(tgross): cut down this time after fixing
		// https://github.com/hashicorp/nomad/issues/7296
	}, 90*time.Second, 5*time.Second)

	// register a volume
	volID := "ebs-vol0"
	vol := &api.CSIVolume{
		ID:             volID,
		Name:           volID,
		ExternalID:     tc.volumeIDs.EBSVolumeID,
		AccessMode:     "single-node-writer",
		AttachmentMode: "file-system",
		PluginID:       "aws-ebs0",
	}
	_, err := nomadClient.CSIVolumes().Register(vol, nil)
	require.NoError(err)
	defer nomadClient.CSIVolumes().Deregister(volID, nil)

	// deploy a job that writes to the volume
	writeJobID := "write-ebs-" + uuid[0:8]
	tc.jobIds = append(tc.jobIds, writeJobID)
	writeAllocs := e2eutil.RegisterAndWaitForAllocs(t, nomadClient,
		"csi/input/use-ebs-volume.nomad", writeJobID, "")
	writeAllocID := writeAllocs[0].ID
	e2eutil.WaitForAllocRunning(t, nomadClient, writeAllocID)

	// read data from volume and assert the writer wrote a file to it
	writeAlloc, _, err := nomadClient.Allocations().Info(writeAllocID, nil)
	require.NoError(err)
	expectedPath := "/local/test/" + writeAllocID
	_, err = readFile(nomadClient, writeAlloc, expectedPath)
	require.NoError(err)

	// Shutdown the writer so we can run a reader.
	// we could mount the EBS volume with multi-attach, but we
	// want this test to exercise the unpublish workflow.
	nomadClient.Jobs().Deregister(writeJobID, true, nil)

	// deploy a job so we can read from the volume
	readJobID := "read-ebs-" + uuid[0:8]
	tc.jobIds = append(tc.jobIds, readJobID)
	readAllocs := e2eutil.RegisterAndWaitForAllocs(t, nomadClient,
		"csi/input/use-ebs-volume.nomad", readJobID, "")
	readAllocID := readAllocs[0].ID
	e2eutil.WaitForAllocRunning(t, nomadClient, readAllocID)

	// ensure we clean up claim before we deregister volumes
	defer nomadClient.Jobs().Deregister(readJobID, true, nil)

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
	tc.jobIds = append(tc.jobIds, nodesJobID)
	e2eutil.RegisterAndWaitForAllocs(t, nomadClient,
		"csi/input/plugin-aws-efs-nodes.nomad", nodesJobID, "")

	// wait for plugin to become healthy
	require.Eventually(func() bool {
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
	}, 90*time.Second, 5*time.Second)

	// register a volume
	volID := "efs-vol0"
	vol := &api.CSIVolume{
		ID:             volID,
		Name:           volID,
		ExternalID:     tc.volumeIDs.EFSVolumeID,
		AccessMode:     "single-node-writer",
		AttachmentMode: "file-system",
		PluginID:       "aws-efs0",
	}
	_, err := nomadClient.CSIVolumes().Register(vol, nil)
	require.NoError(err)
	defer nomadClient.CSIVolumes().Deregister(volID, nil)

	// deploy a job that writes to the volume
	writeJobID := "write-efs-" + uuid[0:8]
	writeAllocs := e2eutil.RegisterAndWaitForAllocs(t, nomadClient,
		"csi/input/use-efs-volume-write.nomad", writeJobID, "")
	writeAllocID := writeAllocs[0].ID
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
	nomadClient.Jobs().Deregister(writeJobID, true, nil)

	// deploy a job that reads from the volume.
	readJobID := "read-efs-" + uuid[0:8]
	readAllocs := e2eutil.RegisterAndWaitForAllocs(t, nomadClient,
		"csi/input/use-efs-volume-read.nomad", readJobID, "")
	defer nomadClient.Jobs().Deregister(readJobID, true, nil)
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
	for _, id := range tc.jobIds {
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

package csi

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/nomad/api"
	e2e "github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/testutil"
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

const ns = ""

var pluginWait = &e2e.WaitConfig{Interval: 5 * time.Second, Retries: 24} // 2min
var reapWait = &e2e.WaitConfig{Interval: 5 * time.Second, Retries: 36}   // 3min

func (tc *CSIVolumesTest) BeforeAll(f *framework.F) {
	t := f.T()

	_, err := os.Stat("csi/input/volume-ebs.hcl")
	if err != nil {
		t.Skip("skipping CSI test because EBS volume spec file missing:", err)
	}

	_, err = os.Stat("csi/input/volume-efs.hcl")
	if err != nil {
		t.Skip("skipping CSI test because EFS volume spec file missing:", err)
	}

	// Ensure cluster has leader and at least two client
	// nodes in a ready state before running tests
	e2e.WaitForLeader(t, tc.Nomad())
	e2e.WaitForNodesReady(t, tc.Nomad(), 2)
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
	pluginID := "aws-ebs0"

	// deploy the controller plugin job
	controllerJobID := "aws-ebs-plugin-controller-" + uuid[0:8]
	f.NoError(e2e.Register(controllerJobID, "csi/input/plugin-aws-ebs-controller.nomad"))
	tc.pluginJobIDs = append(tc.pluginJobIDs, controllerJobID)
	expected := []string{"running", "running"}
	f.NoError(
		e2e.WaitForAllocStatusExpected(controllerJobID, ns, expected),
		"job should be running")

	// deploy the node plugins job
	nodesJobID := "aws-ebs-plugin-nodes-" + uuid[0:8]
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

	f.NoError(waitForPluginStatusControllerCount(pluginID, 2, pluginWait),
		"aws-ebs0 controller plugins did not become healthy")
	f.NoError(waitForPluginStatusMinNodeCount(pluginID, 2, pluginWait),
		"aws-ebs0 node plugins did not become healthy")

	// register a volume
	// TODO: we don't have a unique ID threaded thru the jobspec yet
	volID := "ebs-vol0"
	err := volumeRegister(volID, "csi/input/volume-ebs.hcl")
	require.NoError(err)
	tc.volumeIDs = append(tc.volumeIDs, volID)

	// deploy a job that writes to the volume
	writeJobID := "write-ebs-" + uuid[0:8]
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
	require.NoError(err)

	// Shutdown (and purge) the writer so we can run a reader.
	// we could mount the EBS volume with multi-attach, but we
	// want this test to exercise the unpublish workflow.
	_, err = e2e.Command("nomad", "job", "stop", "-purge", writeJobID)
	require.NoError(err)

	// wait for the volume unpublish workflow to complete
	require.NoError(waitForVolumeClaimRelease(volID, reapWait),
		"write-ebs alloc claim was not released")

	// deploy a job so we can read from the volume
	readJobID := "read-ebs-" + uuid[0:8]
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
	pluginID := "aws-efs0"

	// deploy the node plugins job (no need for a controller for EFS)
	nodesJobID := "aws-efs-plugin-nodes-" + uuid[0:8]
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
		}, nil,
	))

	f.NoError(waitForPluginStatusMinNodeCount(pluginID, 2, pluginWait),
		"aws-efs0 node plugins did not become healthy")

	// register a volume
	volID := "efs-vol0"
	err := volumeRegister(volID, "csi/input/volume-efs.hcl")
	require.NoError(err)
	tc.volumeIDs = append(tc.volumeIDs, volID)

	// deploy a job that writes to the volume
	writeJobID := "write-efs-" + uuid[0:8]
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
	_, err = e2e.Command("nomad", "job", "stop", writeJobID)
	require.NoError(err)

	// wait for the volume unpublish workflow to complete
	require.NoError(waitForVolumeClaimRelease(volID, reapWait),
		"write-efs alloc claim was not released")

	// deploy a job that reads from the volume
	readJobID := "read-efs-" + uuid[0:8]
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

func (tc *CSIVolumesTest) AfterEach(f *framework.F) {

	// Stop all jobs in test
	for _, id := range tc.testJobIDs {
		out, err := e2e.Command("nomad", "job", "stop", "-purge", id)
		f.Assert().NoError(err, out)
	}
	tc.testJobIDs = []string{}

	// Deregister all volumes in test
	for _, id := range tc.volumeIDs {
		// make sure all the test jobs have finished unpublishing claims
		err := waitForVolumeClaimRelease(id, reapWait)
		f.Assert().NoError(err, "volume claims were not released")

		out, err := e2e.Command("nomad", "volume", "deregister", id)
		if err != nil {
			fmt.Println("could not deregister volume, dumping allocation logs")
			f.Assert().NoError(tc.dumpLogs())
		}
		f.Assert().NoError(err, out)
	}
	tc.volumeIDs = []string{}

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

// waitForVolumeClaimRelease makes sure we don't try to re-claim a volume
// that's in the process of being unpublished. we can't just wait for allocs
// to stop, but need to wait for their claims to be released
func waitForVolumeClaimRelease(volID string, wc *e2e.WaitConfig) error {
	var out string
	var err error
	testutil.WaitForResultRetries(wc.Retries, func() (bool, error) {
		time.Sleep(wc.Interval)
		out, err = e2e.Command("nomad", "volume", "status", volID)
		if err != nil {
			return false, err
		}
		section, err := e2e.GetSection(out, "Allocations")
		if err != nil {
			return false, err
		}
		return strings.Contains(section, "No allocations placed"), nil
	}, func(e error) {
		if e == nil {
			err = nil
		}
		err = fmt.Errorf("alloc claim was not released: %v\n%s", e, out)
	})
	return err
}

func (tc *CSIVolumesTest) dumpLogs() error {

	for _, id := range tc.pluginJobIDs {
		allocs, err := e2e.AllocsForJob(id, ns)
		if err != nil {
			return fmt.Errorf("could not find allocs for plugin: %v", err)
		}
		for _, alloc := range allocs {
			allocID := alloc["ID"]
			out, err := e2e.AllocLogs(allocID, e2e.LogsStdErr)
			if err != nil {
				return fmt.Errorf("could not get logs for alloc: %v\n%s", err, out)
			}
			_, isCI := os.LookupEnv("CI")
			if isCI {
				fmt.Println("--------------------------------------")
				fmt.Println("allocation logs:", allocID)
				fmt.Println(out)
				continue
			}
			f, err := os.Create(allocID + ".log")
			if err != nil {
				return fmt.Errorf("could not create log file: %v", err)
			}
			defer f.Close()
			_, err = f.WriteString(out)
			if err != nil {
				return fmt.Errorf("could not write to log file: %v", err)
			}
			fmt.Printf("nomad alloc logs written to %s.log\n", allocID)
		}
	}
	return nil
}

// TODO(tgross): replace this w/ AllocFS().Stat() after
// https://github.com/hashicorp/nomad/issues/7365 is fixed
func readFile(client *api.Client, allocID string, path string) (bytes.Buffer, error) {
	var stdout, stderr bytes.Buffer
	alloc, _, err := client.Allocations().Info(allocID, nil)
	if err != nil {
		return stdout, err
	}
	ctx, cancelFn := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelFn()

	_, err = client.Allocations().Exec(ctx,
		alloc, "task", false,
		[]string{"cat", path},
		os.Stdin, &stdout, &stderr,
		make(chan api.TerminalSize), nil)
	return stdout, err
}

func waitForPluginStatusMinNodeCount(pluginID string, minCount int, wc *e2e.WaitConfig) error {

	return waitForPluginStatusCompare(pluginID, func(out string) (bool, error) {
		expected, err := e2e.GetField(out, "Nodes Expected")
		if err != nil {
			return false, err
		}
		expectedCount, err := strconv.Atoi(strings.TrimSpace(expected))
		if err != nil {
			return false, err
		}
		if expectedCount < minCount {
			return false, fmt.Errorf(
				"expected Nodes Expected >= %d, got %q", minCount, expected)
		}
		healthy, err := e2e.GetField(out, "Nodes Healthy")
		if err != nil {
			return false, err
		}
		if healthy != expected {
			return false, fmt.Errorf(
				"expected Nodes Healthy >= %d, got %q", minCount, healthy)
		}
		return true, nil
	}, wc)
}

func waitForPluginStatusControllerCount(pluginID string, count int, wc *e2e.WaitConfig) error {

	return waitForPluginStatusCompare(pluginID, func(out string) (bool, error) {

		expected, err := e2e.GetField(out, "Controllers Expected")
		if err != nil {
			return false, err
		}
		expectedCount, err := strconv.Atoi(strings.TrimSpace(expected))
		if err != nil {
			return false, err
		}
		if expectedCount != count {
			return false, fmt.Errorf(
				"expected Controllers Expected = %d, got %d", count, expectedCount)
		}
		healthy, err := e2e.GetField(out, "Controllers Healthy")
		if err != nil {
			return false, err
		}
		healthyCount, err := strconv.Atoi(strings.TrimSpace(healthy))
		if err != nil {
			return false, err
		}
		if healthyCount != count {
			return false, fmt.Errorf(
				"expected Controllers Healthy = %d, got %d", count, healthyCount)
		}
		return true, nil

	}, wc)
}

func waitForPluginStatusCompare(pluginID string, compare func(got string) (bool, error), wc *e2e.WaitConfig) error {
	var err error
	testutil.WaitForResultRetries(wc.Retries, func() (bool, error) {
		time.Sleep(wc.Interval)
		out, err := e2e.Command("nomad", "plugin", "status", pluginID)
		if err != nil {
			return false, err
		}
		return compare(out)
	}, func(e error) {
		err = fmt.Errorf("plugin status check failed: %v", e)
	})
	return err
}

// VolumeRegister registers a jobspec from a file but with a unique ID.
// The caller is responsible for recording that ID for later cleanup.
func volumeRegister(volID, volFilePath string) error {

	cmd := exec.Command("nomad", "volume", "register", "-")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("could not open stdin?: %w", err)
	}

	content, err := ioutil.ReadFile(volFilePath)
	if err != nil {
		return fmt.Errorf("could not open vol file: %w", err)
	}

	// hack off the first line to replace with our unique ID
	var re = regexp.MustCompile(`(?m)^id ".*"`)
	volspec := re.ReplaceAllString(string(content),
		fmt.Sprintf("id = \"%s\"", volID))

	go func() {
		defer stdin.Close()
		io.WriteString(stdin, volspec)
	}()

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not register vol: %w\n%v", err, string(out))
	}
	return nil
}

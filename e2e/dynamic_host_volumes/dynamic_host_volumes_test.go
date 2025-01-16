// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dynamic_host_volumes

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/v3/jobs3"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
)

// TestDynamicHostVolumes_CreateWorkflow tests the workflow where a dynamic host
// volume is created by a plugin and then mounted by a job.
func TestDynamicHostVolumes_CreateWorkflow(t *testing.T) {

	start := time.Now()
	nomad := e2eutil.NomadClient(t)
	e2eutil.WaitForLeader(t, nomad)
	e2eutil.WaitForNodesReady(t, nomad, 1)

	out, err := e2eutil.Command("nomad", "volume", "create",
		"-detach", "input/volume-create.nomad.hcl")
	must.NoError(t, err)

	split := strings.Split(out, " ")
	volID := strings.TrimSpace(split[len(split)-1])
	t.Logf("[%v] volume %q created", time.Since(start), volID)

	t.Cleanup(func() {
		_, err := e2eutil.Command("nomad", "volume", "delete", "-type", "host", volID)
		must.NoError(t, err)
	})

	out, err = e2eutil.Command("nomad", "volume", "status", "-type", "host", volID)
	must.NoError(t, err)

	nodeID, err := e2eutil.GetField(out, "Node ID")
	must.NoError(t, err)
	must.NotEq(t, "", nodeID)
	t.Logf("[%v] waiting for volume %q to be ready", time.Since(start), volID)

	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(func() error {
			node, _, err := nomad.Nodes().Info(nodeID, nil)
			if err != nil {
				return err
			}
			_, ok := node.HostVolumes["created-volume"]
			if !ok {
				return fmt.Errorf("node %q did not fingerprint volume %q", nodeID, volID)
			}
			vol, _, err := nomad.HostVolumes().Get(volID, nil)
			if err != nil {
				return err
			}
			if vol.State != "ready" {
				return fmt.Errorf("node fingerprinted volume but status was not updated")
			}
			t.Logf("[%v] volume %q is ready", time.Since(start), volID)
			return nil
		}),
		wait.Timeout(10*time.Second),
		wait.Gap(50*time.Millisecond),
	))

	t.Logf("[%v] submitting mounter job", time.Since(start))
	_, cleanup := jobs3.Submit(t, "./input/mount-created.nomad.hcl")
	t.Cleanup(cleanup)
	t.Logf("[%v] test complete, cleaning up", time.Since(start))
}

// TestDynamicHostVolumes_RegisterWorkflow tests the workflow where a dynamic
// host volume is created out-of-band and registered by a job, then mounted by
// another job.
func TestDynamicHostVolumes_RegisterWorkflow(t *testing.T) {

	start := time.Now()
	nomad := e2eutil.NomadClient(t)
	e2eutil.WaitForLeader(t, nomad)
	e2eutil.WaitForNodesReady(t, nomad, 1)

	submitted, cleanup := jobs3.Submit(t, "./input/register-volumes.nomad.hcl",
		jobs3.Dispatcher(),
	)
	t.Cleanup(cleanup)
	t.Logf("[%v] register job %q created", time.Since(start), submitted.JobID())

	_, err := e2eutil.Command(
		"nomad", "acl", "policy", "apply",
		"-namespace", "default", "-job", submitted.JobID(),
		"register-volumes-policy", "./input/register-volumes.policy.hcl")
	must.NoError(t, err)
	t.Logf("[%v] ACL policy for job %q created", time.Since(start), submitted.JobID())

	must.NoError(t, e2eutil.Dispatch(submitted.JobID(),
		map[string]string{
			"vol_name": "registered-volume",
			"vol_size": "1G",
			"vol_path": "/tmp/registered-volume",
		}, ""))
	t.Logf("[%v] job dispatched", time.Since(start))

	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(func() error {
			dispatches, err := e2eutil.DispatchedJobs(submitted.JobID())
			if err != nil {
				return err
			}
			if len(dispatches) == 0 {
				return fmt.Errorf("no dispatched job for %q", submitted.JobID())
			}

			jobID := dispatches[0]["ID"]
			must.NotEq(t, "", jobID,
				must.Sprintf("invalid dispatched jobs output: %v", dispatches))

			allocs, _, err := nomad.Jobs().Allocations(jobID, true, nil)
			if len(allocs) == 0 || allocs[0].ClientStatus != "complete" {
				out, _ := e2eutil.AllocLogs(allocs[0].ID, "default", e2eutil.LogsStdErr)
				return fmt.Errorf("allocation status was %q. logs: %s",
					allocs[0].ClientStatus, out)
			}

			t.Logf("[%v] dispatched job done", time.Since(start))
			return nil
		}),
		wait.Timeout(10*time.Second),
		wait.Gap(50*time.Millisecond),
	))

	out, err := e2eutil.Command("nomad", "volume", "status", "-verbose", "-type", "host")
	must.NoError(t, err)
	vols, err := e2eutil.ParseColumns(out)
	must.NoError(t, err)

	var volID string
	var nodeID string
	for _, vol := range vols {
		if vol["Name"] == "registered-volume" {
			volID = vol["ID"]
			nodeID = vol["Node ID"]
		}
	}
	must.NotEq(t, "", volID, must.Sprintf("volume was not registered: %s", out))

	t.Cleanup(func() {
		_, err := e2eutil.Command("nomad", "volume", "delete", "-type", "host", volID)
		must.NoError(t, err)
	})

	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(func() error {
			node, _, err := nomad.Nodes().Info(nodeID, nil)
			if err != nil {
				return err
			}
			_, ok := node.HostVolumes["registered-volume"]
			if !ok {
				return fmt.Errorf("node %q did not fingerprint volume %q", nodeID, volID)
			}
			vol, _, err := nomad.HostVolumes().Get(volID, nil)
			if err != nil {
				return err
			}
			if vol.State != "ready" {
				return fmt.Errorf("node fingerprinted volume but status was not updated")
			}

			t.Logf("[%v] volume %q is ready", time.Since(start), volID)
			return nil
		}),
		wait.Timeout(10*time.Second),
		wait.Gap(50*time.Millisecond),
	))

	t.Logf("[%v] submitting mounter job", time.Since(start))
	_, cleanup2 := jobs3.Submit(t, "./input/mount-registered.nomad.hcl")
	t.Cleanup(cleanup2)
	t.Logf("[%v] test complete, cleaning up", time.Since(start))
}

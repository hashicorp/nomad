// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dynamic_host_volumes

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	nomadapi "github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/v3/jobs3"
	"github.com/hashicorp/nomad/e2e/v3/volumes3"
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

	_, cleanupVol := volumes3.Create(t, "input/volume-create.nomad.hcl",
		volumes3.WithClient(nomad))
	t.Cleanup(cleanupVol)

	t.Logf("[%v] submitting mounter job", time.Since(start))
	_, cleanupJob := jobs3.Submit(t, "./input/mount-created.nomad.hcl")
	t.Cleanup(cleanupJob)
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

	section, err := e2eutil.GetSection(out, "Dynamic Host Volumes")
	must.NoError(t, err)
	vols, err := e2eutil.ParseColumns(section)
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

// TestDynamicHostVolumes_StickyVolumes tests where a job marks a volume as
// sticky and its allocations should have strong associations with specific
// volumes as they are replaced
func TestDynamicHostVolumes_StickyVolumes(t *testing.T) {

	start := time.Now()
	nomad := e2eutil.NomadClient(t)
	e2eutil.WaitForLeader(t, nomad)
	e2eutil.WaitForNodesReady(t, nomad, 2)

	// TODO: if we create # of volumes == # of nodes, we can make test flakes
	// stand out more easily

	vol1Sub, cleanup1 := volumes3.Create(t, "input/volume-sticky.nomad.hcl",
		volumes3.WithClient(nomad))
	t.Cleanup(cleanup1)

	vol2Sub, cleanup2 := volumes3.Create(t, "input/volume-sticky.nomad.hcl",
		volumes3.WithClient(nomad))
	t.Cleanup(cleanup2)

	nodeToVolMap := map[string]string{
		vol1Sub.NodeID(): vol1Sub.VolumeID(),
		vol2Sub.NodeID(): vol2Sub.VolumeID(),
	}

	t.Logf("[%v] submitting sticky volume mounter job", time.Since(start))
	jobSub, cleanupJob := jobs3.Submit(t, "./input/sticky.nomad.hcl")
	t.Cleanup(cleanupJob)

	allocID1 := jobSub.Allocs()[0].ID
	alloc, _, err := nomad.Allocations().Info(allocID1, nil)
	must.NoError(t, err)

	selectedNodeID := alloc.NodeID
	selectedVolID := nodeToVolMap[selectedNodeID]

	t.Logf("[%v] volume %q on node %q was selected",
		time.Since(start), selectedVolID, selectedNodeID)

	// Test: force reschedule

	_, err = nomad.Allocations().Stop(alloc, nil)
	must.NoError(t, err)

	t.Logf("[%v] stopped allocation %q", time.Since(start), alloc.ID)

	var allocID2 string

	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(func() error {
			allocs, _, err := nomad.Jobs().Allocations(jobSub.JobID(), true, nil)
			must.NoError(t, err)
			if len(allocs) != 2 {
				return fmt.Errorf("alloc not started")
			}
			for _, a := range allocs {
				if a.ID != allocID1 {
					allocID2 = a.ID
					if a.ClientStatus != api.AllocClientStatusRunning {
						return fmt.Errorf("replacement alloc not running")
					}
				}
			}
			return nil
		}),
		wait.Timeout(10*time.Second),
		wait.Gap(50*time.Millisecond),
	))

	newAlloc, _, err := nomad.Allocations().Info(allocID2, nil)
	must.NoError(t, err)
	must.Eq(t, selectedNodeID, newAlloc.NodeID)
	t.Logf("[%v] replacement alloc %q is running", time.Since(start), newAlloc.ID)

	// Test: drain node

	t.Logf("[%v] draining node %q", time.Since(start), selectedNodeID)
	cleanup, err := drainNode(nomad, selectedNodeID, time.Second*20)
	t.Cleanup(cleanup)
	must.NoError(t, err)

	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(func() error {
			evals, _, err := nomad.Jobs().Evaluations(jobSub.JobID(), nil)
			if err != nil {
				return err
			}

			got := map[string]string{}

			for _, eval := range evals {
				got[eval.ID[:8]] = fmt.Sprintf("status=%q trigger=%q create_index=%d",
					eval.Status,
					eval.TriggeredBy,
					eval.CreateIndex,
				)
				if eval.Status == nomadapi.EvalStatusBlocked {
					return nil
				}
			}

			return fmt.Errorf("expected blocked eval, got evals => %#v", got)
		}),
		wait.Timeout(10*time.Second),
		wait.Gap(50*time.Millisecond),
	))

	t.Logf("[%v] undraining node %q", time.Since(start), selectedNodeID)
	cleanup()

	var allocID3 string
	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(func() error {
			allocs, _, err := nomad.Jobs().Allocations(jobSub.JobID(), true, nil)
			must.NoError(t, err)
			if len(allocs) != 3 {
				return fmt.Errorf("alloc not started")
			}
			for _, a := range allocs {
				if a.ID != allocID1 && a.ID != allocID2 {
					allocID3 = a.ID
					if a.ClientStatus != api.AllocClientStatusRunning {
						return fmt.Errorf("replacement alloc %q not running", allocID3)
					}
				}
			}
			return nil
		}),
		wait.Timeout(10*time.Second),
		wait.Gap(50*time.Millisecond),
	))

	newAlloc, _, err = nomad.Allocations().Info(allocID3, nil)
	must.NoError(t, err)
	must.Eq(t, selectedNodeID, newAlloc.NodeID)
	t.Logf("[%v] replacement alloc %q is running", time.Since(start), newAlloc.ID)

}

func drainNode(nomad *nomadapi.Client, nodeID string, timeout time.Duration) (func(), error) {
	resp, err := nomad.Nodes().UpdateDrainOpts(nodeID, &nomadapi.DrainOptions{
		DrainSpec:    &nomadapi.DrainSpec{},
		MarkEligible: false,
	}, nil)
	if err != nil {
		return func() {}, err
	}

	cleanup := func() {
		nomad.Nodes().UpdateDrainOpts(nodeID, &nomadapi.DrainOptions{
			MarkEligible: true}, nil)
	}

	ctx, cancel := context.WithTimeout(context.TODO(), timeout)
	defer cancel()
	drainCh := nomad.Nodes().MonitorDrain(ctx, nodeID, resp.EvalCreateIndex, false)

	for {
		select {
		case <-ctx.Done():
			return cleanup, err
		case msg := <-drainCh:
			if msg == nil {
				return cleanup, nil
			}
		}
	}
}

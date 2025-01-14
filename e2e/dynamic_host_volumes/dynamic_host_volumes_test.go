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

	nomad := e2eutil.NomadClient(t)
	e2eutil.WaitForLeader(t, nomad)
	e2eutil.WaitForNodesReady(t, nomad, 1)

	out, err := e2eutil.Command("nomad", "volume", "create",
		"-detach", "input/volume-create.nomad.hcl")
	must.NoError(t, err)

	split := strings.Split(out, " ")
	volID := strings.TrimSpace(split[len(split)-1])
	t.Logf("created volume: %q\n", volID)

	t.Cleanup(func() {
		_, err := e2eutil.Command("nomad", "volume", "delete", "-type", "host", volID)
		must.NoError(t, err)
	})

	out, err = e2eutil.Command("nomad", "volume", "status", "-type", "host", volID)
	must.NoError(t, err)

	nodeID, err := e2eutil.GetField(out, "Node ID")
	must.NoError(t, err)

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
			return nil
		}),
		wait.Timeout(5*time.Second),
		wait.Gap(50*time.Millisecond),
	))

	_, cleanup := jobs3.Submit(t, "./input/mount-created.nomad.hcl")
	t.Cleanup(cleanup)
}

// TestDynamicHostVolumes_RegisterWorkflow tests the workflow where a dynamic
// host volume is created out-of-band and registered by a job, then mounted by
// another job.
func TestDynamicHostVolumes_RegisterWorkflow(t *testing.T) {

	nomad := e2eutil.NomadClient(t)
	e2eutil.WaitForLeader(t, nomad)
	e2eutil.WaitForNodesReady(t, nomad, 1)

	submitted, cleanup := jobs3.Submit(t, "./input/register-volumes.nomad.hcl",
		jobs3.Dispatcher(),
	)
	t.Cleanup(cleanup)

	_, err := e2eutil.Command(
		"nomad", "acl", "policy", "apply",
		"-namespace", "default", "-job", submitted.JobID(),
		"register-volumes-policy", "./input/register-volumes.policy.hcl")
	must.NoError(t, err)

	must.NoError(t, e2eutil.Dispatch(submitted.JobID(),
		map[string]string{
			"vol_name": "registered-volume",
			"vol_size": "1G",
			"vol_path": "/tmp/registered-volume",
		}, ""))

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
	must.NotEq(t, "", volID)

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
			return nil
		}),
		wait.Timeout(5*time.Second),
		wait.Gap(50*time.Millisecond),
	))

	_, cleanup2 := jobs3.Submit(t, "./input/mount-registered.nomad.hcl")
	t.Cleanup(cleanup2)

}

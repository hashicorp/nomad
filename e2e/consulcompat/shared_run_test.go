// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consulcompat

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	consulapi "github.com/hashicorp/consul/api"
	nomadapi "github.com/hashicorp/nomad/api"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
)

// verifyConsulVersion ensures that we've successfully spun up a Consul cluster
// on the expected version (this ensures we don't have stray running Consul from
// previous runs or from the development environment)
func verifyConsulVersion(t *testing.T, consulAPI *consulapi.Client, version string) {
	self, err := consulAPI.Agent().Self()
	must.NoError(t, err)
	vers := self["Config"]["Version"].(string)
	must.Eq(t, version, vers)
}

// verifyConsulFingerprint ensures that we've successfully fingerprinted Consul
func verifyConsulFingerprint(t *testing.T, nc *nomadapi.Client, version, clusterName string) {
	stubs, _, err := nc.Nodes().List(nil)
	must.NoError(t, err)
	must.Len(t, 1, stubs)
	node, _, err := nc.Nodes().Info(stubs[0].ID, nil)

	if clusterName == "default" {
		must.Eq(t, version, node.Attributes["consul.version"])
	} else {
		must.Eq(t, version, node.Attributes["consul."+clusterName+".version"])
	}
}

func runConnectJob(t *testing.T, nc *nomadapi.Client) {

	b, err := os.ReadFile("./input/connect.nomad.hcl")
	must.NoError(t, err)

	jobs := nc.Jobs()
	job, err := jobs.ParseHCL(string(b), true)
	must.NoError(t, err, must.Sprint("failed to parse job HCL"))

	resp, _, err := jobs.Register(job, nil)
	must.NoError(t, err, must.Sprint("failed to register job"))
	evalID := resp.EvalID
	t.Logf("eval: %s", evalID)

	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(func() error {
			eval, _, err := nc.Evaluations().Info(evalID, nil)
			must.NoError(t, err)
			if eval.Status == "complete" {
				// if we have failed allocations it can be difficult to debug in
				// CI, so dump the struct values here so they show up in the
				// logs
				must.MapEmpty(t, eval.FailedTGAllocs,
					must.Sprintf("api=>%#v dash=>%#v",
						eval.FailedTGAllocs["api"], eval.FailedTGAllocs["dashboard"]))
				return nil
			} else {
				return fmt.Errorf("eval is not complete: %s", eval.Status)
			}
		}),
		wait.Timeout(time.Second),
		wait.Gap(100*time.Millisecond),
	))

	t.Cleanup(func() {
		_, _, err = jobs.Deregister(*job.Name, true, nil)
		must.NoError(t, err, must.Sprint("failed to deregister job"))

		must.Wait(t, wait.InitialSuccess(
			wait.ErrorFunc(func() error {
				allocs, _, err := jobs.Allocations(*job.ID, false, nil)
				if err != nil {
					return err
				}
				for _, alloc := range allocs {
					if alloc.ClientStatus == "running" {
						return fmt.Errorf("expected alloc %s to be stopped", alloc.ID)
					}
				}
				return nil
			}),
			wait.Timeout(30*time.Second),
			wait.Gap(1*time.Second),
		))

		// give Nomad time to sync Consul before shutdown
		time.Sleep(3 * time.Second)
	})

	var dashboardAllocID string

	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(func() error {
			allocs, _, err := jobs.Allocations(*job.ID, false, nil)
			if err != nil {
				return err
			}
			if n := len(allocs); n != 2 {
				return fmt.Errorf("expected 2 alloc, got %d", n)
			}
			for _, alloc := range allocs {
				if alloc.TaskGroup == "dashboard" {
					dashboardAllocID = alloc.ID // save for later
				}
				if alloc.ClientStatus != "running" {
					return fmt.Errorf(
						"expected alloc status running, got %s for %s",
						alloc.ClientStatus, alloc.ID)
				}
			}
			return nil
		}),
		wait.Timeout(30*time.Second),
		wait.Gap(1*time.Second),
	))

	// Ensure that the dashboard is reachable and can connect to the API
	alloc, _, err := nc.Allocations().Info(dashboardAllocID, nil)
	must.NoError(t, err)

	network := alloc.AllocatedResources.Shared.Networks[0]
	dynPort := network.DynamicPorts[0]
	addr := fmt.Sprintf("http://%s:%d", network.IP, dynPort.Value)

	// the alloc may be running but not yet listening, so give it a few seconds
	// to start up
	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(func() error {
			info, err := http.Get(addr)
			if err != nil {
				return err
			}
			defer info.Body.Close()
			body, _ := io.ReadAll(info.Body)

			if !strings.Contains(string(body), "Dashboard") {
				return fmt.Errorf("expected body to contain \"Dashboard\"")
			}
			return nil
		}),
		wait.Timeout(10*time.Second),
		wait.Gap(1*time.Second),
	))
}

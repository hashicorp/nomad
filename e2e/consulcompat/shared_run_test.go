// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consulcompat

import (
	"fmt"
	"io"
	"net/http"
	"os"
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
	})

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
	allocs, _, err := jobs.Allocations(*job.ID, false, nil)
	must.NoError(t, err)
	for _, alloc := range allocs {
		if alloc.TaskGroup == "dashboard" {
			resp, err := http.Get("http://127.0.0.1:9002")
			must.NoError(t, err)
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			t.Logf(string(body))
		}
	}

}

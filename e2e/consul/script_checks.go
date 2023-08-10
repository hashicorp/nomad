// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	capi "github.com/hashicorp/consul/api"
	napi "github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/stretchr/testify/require"
)

type ScriptChecksE2ETest struct {
	framework.TC
	jobIds []string
}

func (tc *ScriptChecksE2ETest) BeforeAll(f *framework.F) {
	// Ensure cluster has leader before running tests
	e2eutil.WaitForLeader(f.T(), tc.Nomad())
	// Ensure that we have at least 1 client node in ready state
	e2eutil.WaitForNodesReady(f.T(), tc.Nomad(), 1)
}

// TestGroupScriptCheck runs a job with a single task group with several services
// and associated script checks. It updates, stops, etc. the job to verify
// that script checks are re-registered as expected.
func (tc *ScriptChecksE2ETest) TestGroupScriptCheck(f *framework.F) {
	r := require.New(f.T())

	nomadClient := tc.Nomad()
	consulClient := tc.Consul()

	jobId := "checks_group" + uuid.Short()
	tc.jobIds = append(tc.jobIds, jobId)

	// Job run: verify that checks were registered in Consul
	allocs := e2eutil.RegisterAndWaitForAllocs(f.T(),
		nomadClient, "consul/input/checks_group.nomad", jobId, "")
	r.Equal(1, len(allocs))
	e2eutil.RequireConsulStatus(r, consulClient, consulNamespace, "group-service-1", capi.HealthPassing)
	e2eutil.RequireConsulStatus(r, consulClient, consulNamespace, "group-service-2", capi.HealthWarning)
	e2eutil.RequireConsulStatus(r, consulClient, consulNamespace, "group-service-3", capi.HealthCritical)

	// Check in warning state becomes healthy after check passes
	_, _, err := exec(nomadClient, allocs,
		[]string{"/bin/sh", "-c", "touch /tmp/${NOMAD_ALLOC_ID}-alive-2b"})
	r.NoError(err)
	e2eutil.RequireConsulStatus(r, consulClient, consulNamespace, "group-service-2", capi.HealthPassing)

	// Job update: verify checks are re-registered in Consul
	allocs = e2eutil.RegisterAndWaitForAllocs(f.T(),
		nomadClient, "consul/input/checks_group_update.nomad", jobId, "")
	r.Equal(1, len(allocs))
	e2eutil.RequireConsulStatus(r, consulClient, consulNamespace, "group-service-1", capi.HealthPassing)
	e2eutil.RequireConsulStatus(r, consulClient, consulNamespace, "group-service-2", capi.HealthPassing)
	e2eutil.RequireConsulStatus(r, consulClient, consulNamespace, "group-service-3", capi.HealthCritical)

	// Verify we don't have any linger script checks running on the client
	out, _, err := exec(nomadClient, allocs, []string{"pgrep", "sleep"})
	r.NoError(err)
	running := strings.Split(strings.TrimSpace(out.String()), "\n")
	r.LessOrEqual(len(running), 2) // task itself + 1 check == 2

	// Clean job stop: verify that checks were deregistered in Consul
	_, _, err = nomadClient.Jobs().Deregister(jobId, false, nil) // nomad job stop
	r.NoError(err)
	e2eutil.RequireConsulDeregistered(r, consulClient, consulNamespace, "group-service-1")
	e2eutil.RequireConsulDeregistered(r, consulClient, consulNamespace, "group-service-2")
	e2eutil.RequireConsulDeregistered(r, consulClient, consulNamespace, "group-service-3")

	// Restore for next test
	allocs = e2eutil.RegisterAndWaitForAllocs(f.T(),
		nomadClient, "consul/input/checks_group.nomad", jobId, "")
	r.Equal(2, len(allocs))
	e2eutil.RequireConsulStatus(r, consulClient, consulNamespace, "group-service-1", capi.HealthPassing)
	e2eutil.RequireConsulStatus(r, consulClient, consulNamespace, "group-service-2", capi.HealthWarning)
	e2eutil.RequireConsulStatus(r, consulClient, consulNamespace, "group-service-3", capi.HealthCritical)

	// Crash a task: verify that checks become healthy again
	_, _, err = exec(nomadClient, allocs, []string{"pkill", "sleep"})
	if err != nil && err.Error() != "plugin is shut down" {
		r.FailNow("unexpected error: %v", err)
	}
	e2eutil.RequireConsulStatus(r, consulClient, consulNamespace, "group-service-1", capi.HealthPassing)
	e2eutil.RequireConsulStatus(r, consulClient, consulNamespace, "group-service-2", capi.HealthWarning)
	e2eutil.RequireConsulStatus(r, consulClient, consulNamespace, "group-service-3", capi.HealthCritical)

	// TODO(tgross) ...
	// Restart client: verify that checks are re-registered
}

// TestTaskScriptCheck runs a job with a single task with several services
// and associated script checks. It updates, stops, etc. the job to verify
// that script checks are re-registered as expected.
func (tc *ScriptChecksE2ETest) TestTaskScriptCheck(f *framework.F) {
	r := require.New(f.T())

	nomadClient := tc.Nomad()
	consulClient := tc.Consul()

	jobId := "checks_task" + uuid.Short()
	tc.jobIds = append(tc.jobIds, jobId)

	// Job run: verify that checks were registered in Consul
	allocs := e2eutil.RegisterAndWaitForAllocs(f.T(),
		nomadClient, "consul/input/checks_task.nomad", jobId, "")
	r.Equal(1, len(allocs))
	e2eutil.RequireConsulStatus(r, consulClient, consulNamespace, "task-service-1", capi.HealthPassing)
	e2eutil.RequireConsulStatus(r, consulClient, consulNamespace, "task-service-2", capi.HealthWarning)
	e2eutil.RequireConsulStatus(r, consulClient, consulNamespace, "task-service-3", capi.HealthCritical)

	// Check in warning state becomes healthy after check passes
	_, _, err := exec(nomadClient, allocs,
		[]string{"/bin/sh", "-c", "touch ${NOMAD_TASK_DIR}/alive-2b"})
	r.NoError(err)
	e2eutil.RequireConsulStatus(r, consulClient, consulNamespace, "task-service-2", capi.HealthPassing)

	// Job update: verify checks are re-registered in Consul
	allocs = e2eutil.RegisterAndWaitForAllocs(f.T(),
		nomadClient, "consul/input/checks_task_update.nomad", jobId, "")
	r.Equal(1, len(allocs))
	e2eutil.RequireConsulStatus(r, consulClient, consulNamespace, "task-service-1", capi.HealthPassing)
	e2eutil.RequireConsulStatus(r, consulClient, consulNamespace, "task-service-2", capi.HealthPassing)
	e2eutil.RequireConsulStatus(r, consulClient, consulNamespace, "task-service-3", capi.HealthCritical)

	// Verify we don't have any linger script checks running on the client
	out, _, err := exec(nomadClient, allocs, []string{"pgrep", "sleep"})
	r.NoError(err)
	running := strings.Split(strings.TrimSpace(out.String()), "\n")
	r.LessOrEqual(len(running), 2) // task itself + 1 check == 2

	// Clean job stop: verify that checks were deregistered in Consul
	_, _, err = nomadClient.Jobs().Deregister(jobId, false, nil) // nomad job stop
	r.NoError(err)
	e2eutil.RequireConsulDeregistered(r, consulClient, consulNamespace, "task-service-1")
	e2eutil.RequireConsulDeregistered(r, consulClient, consulNamespace, "task-service-2")
	e2eutil.RequireConsulDeregistered(r, consulClient, consulNamespace, "task-service-3")

	// Restore for next test
	allocs = e2eutil.RegisterAndWaitForAllocs(f.T(),
		nomadClient, "consul/input/checks_task.nomad", jobId, "")
	r.Equal(2, len(allocs))
	e2eutil.RequireConsulStatus(r, consulClient, consulNamespace, "task-service-1", capi.HealthPassing)
	e2eutil.RequireConsulStatus(r, consulClient, consulNamespace, "task-service-2", capi.HealthWarning)
	e2eutil.RequireConsulStatus(r, consulClient, consulNamespace, "task-service-3", capi.HealthCritical)

	// Crash a task: verify that checks become healthy again
	_, _, err = exec(nomadClient, allocs, []string{"pkill", "sleep"})
	if err != nil && err.Error() != "plugin is shut down" {
		r.FailNow("unexpected error: %v", err)
	}
	e2eutil.RequireConsulStatus(r, consulClient, consulNamespace, "task-service-1", capi.HealthPassing)
	e2eutil.RequireConsulStatus(r, consulClient, consulNamespace, "task-service-2", capi.HealthWarning)
	e2eutil.RequireConsulStatus(r, consulClient, consulNamespace, "task-service-3", capi.HealthCritical)

	// TODO(tgross) ...
	// Restart client: verify that checks are re-registered
}

func (tc *ScriptChecksE2ETest) AfterEach(f *framework.F) {
	r := require.New(f.T())

	nomadClient := tc.Nomad()
	jobs := nomadClient.Jobs()
	// Stop all jobs in test
	for _, id := range tc.jobIds {
		_, _, err := jobs.Deregister(id, true, nil)
		r.NoError(err)
	}
	// Garbage collect
	r.NoError(nomadClient.System().GarbageCollect())
}

func exec(client *napi.Client, allocs []*napi.AllocationListStub, command []string) (bytes.Buffer, bytes.Buffer, error) {
	ctx, cancelFn := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelFn()

	// we're getting a list of from the registration call here but
	// one of them might be stopped or stopping, which will return
	// an error if we try to exec into it.
	var alloc *napi.Allocation
	for _, stub := range allocs {
		if stub.DesiredStatus == "run" {
			alloc = &napi.Allocation{
				ID:        stub.ID,
				Namespace: stub.Namespace,
				NodeID:    stub.NodeID,
			}
		}
	}
	var stdout, stderr bytes.Buffer
	if alloc == nil {
		return stdout, stderr, fmt.Errorf("no allocation ready for exec")
	}
	_, err := client.Allocations().Exec(ctx,
		alloc, "test", false,
		command,
		os.Stdin, &stdout, &stderr,
		make(chan napi.TerminalSize), nil)
	return stdout, stderr, err
}

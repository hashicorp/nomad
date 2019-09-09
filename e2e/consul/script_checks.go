package consul

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	capi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/api"
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

// requireStatus asserts the aggregate health of the service converges to
// the expected status
func requireStatus(require *require.Assertions,
	consulClient *capi.Client, serviceName, expectedStatus string) {
	require.Eventually(func() bool {
		_, status := serviceStatus(require, consulClient, serviceName)
		return status == expectedStatus
	}, 30*time.Second, time.Second, // needs a long time for killing tasks/clients
		"timed out expecting %q to become %q",
		serviceName, expectedStatus,
	)
}

// serviceStatus gets the aggregate health of the service and returns
// the []ServiceEntry for further checking
func serviceStatus(require *require.Assertions,
	consulClient *capi.Client, serviceName string) ([]*capi.ServiceEntry, string) {
	services, _, err := consulClient.Health().Service(serviceName, "", false, nil)
	require.NoError(err, "expected no error for %q, got %v", serviceName, err)
	if len(services) > 0 {
		return services, services[0].Checks.AggregatedStatus()
	}
	return nil, "(unknown status)"
}

// requireDeregistered asserts that the service eventually is deregistered from Consul
func requireDeregistered(require *require.Assertions,
	consulClient *capi.Client, serviceName string) {
	require.Eventually(func() bool {
		services, _, err := consulClient.Health().Service(serviceName, "", false, nil)
		require.NoError(err, "expected no error for %q, got %v", serviceName, err)
		return len(services) == 0
	}, 5*time.Second, time.Second)
}

// TestGroupScriptCheck runs a job with a single task group with several services
// and associated script checks. It updates, stops, etc. the job to verify
// that script checks are re-registered as expected.
func (tc *ScriptChecksE2ETest) TestGroupScriptCheck(f *framework.F) {
	nomadClient := tc.Nomad()
	uuid := uuid.Generate()
	require := require.New(f.T())
	consulClient := tc.Consul()

	jobId := "checks_group" + uuid[0:8]
	tc.jobIds = append(tc.jobIds, jobId)

	// Job run: verify that checks were registered in Consul
	allocs := e2eutil.RegisterAndWaitForAllocs(f.T(),
		nomadClient, "consul/input/checks_group.nomad", jobId)
	require.Equal(1, len(allocs))
	requireStatus(require, consulClient, "group-service-1", capi.HealthPassing)
	requireStatus(require, consulClient, "group-service-2", capi.HealthWarning)
	requireStatus(require, consulClient, "group-service-3", capi.HealthCritical)

	// Check in warning state becomes healthy after check passes
	_, _, err := exec(nomadClient, allocs,
		[]string{"/bin/sh", "-c", "touch ${NOMAD_TASK_DIR}/alive-2b"})
	require.NoError(err)
	requireStatus(require, consulClient, "group-service-2", capi.HealthPassing)

	// Job update: verify checks are re-registered in Consul
	allocs = e2eutil.RegisterAndWaitForAllocs(f.T(),
		nomadClient, "consul/input/checks_group_update.nomad", jobId)
	require.Equal(1, len(allocs))
	requireStatus(require, consulClient, "group-service-1", capi.HealthPassing)
	requireStatus(require, consulClient, "group-service-2", capi.HealthPassing)
	requireStatus(require, consulClient, "group-service-3", capi.HealthCritical)

	// Verify we don't have any linger script checks running on the client
	out, _, err := exec(nomadClient, allocs, []string{"pgrep", "sleep"})
	require.NoError(err)
	running := strings.Split(strings.TrimSpace(out.String()), "\n")
	require.LessOrEqual(len(running), 2) // task itself + 1 check == 2

	// Clean job stop: verify that checks were deregistered in Consul
	nomadClient.Jobs().Deregister(jobId, false, nil) // nomad job stop
	requireDeregistered(require, consulClient, "group-service-1")
	requireDeregistered(require, consulClient, "group-service-2")
	requireDeregistered(require, consulClient, "group-service-3")

	// Restore for next test
	allocs = e2eutil.RegisterAndWaitForAllocs(f.T(),
		nomadClient, "consul/input/checks_group.nomad", jobId)
	require.Equal(2, len(allocs))
	requireStatus(require, consulClient, "group-service-1", capi.HealthPassing)
	requireStatus(require, consulClient, "group-service-2", capi.HealthWarning)
	requireStatus(require, consulClient, "group-service-3", capi.HealthCritical)

	// Crash a task: verify that checks become healthy again
	_, _, err = exec(nomadClient, allocs, []string{"pkill", "sleep"})
	if err != nil && err.Error() != "plugin is shut down" {
		require.FailNow("unexpected error: %v", err)
	}
	requireStatus(require, consulClient, "group-service-1", capi.HealthPassing)
	requireStatus(require, consulClient, "group-service-2", capi.HealthWarning)
	requireStatus(require, consulClient, "group-service-3", capi.HealthCritical)

	// TODO(tgross) ...
	// Restart client: verify that checks are re-registered
}

// TestTaskScriptCheck runs a job with a single task with several services
// and associated script checks. It updates, stops, etc. the job to verify
// that script checks are re-registered as expected.
func (tc *ScriptChecksE2ETest) TestTaskScriptCheck(f *framework.F) {
	nomadClient := tc.Nomad()
	uuid := uuid.Generate()
	require := require.New(f.T())
	consulClient := tc.Consul()

	jobId := "checks_task" + uuid[0:8]
	tc.jobIds = append(tc.jobIds, jobId)

	// Job run: verify that checks were registered in Consul
	allocs := e2eutil.RegisterAndWaitForAllocs(f.T(),
		nomadClient, "consul/input/checks_task.nomad", jobId)
	require.Equal(1, len(allocs))
	requireStatus(require, consulClient, "task-service-1", capi.HealthPassing)
	requireStatus(require, consulClient, "task-service-2", capi.HealthWarning)
	requireStatus(require, consulClient, "task-service-3", capi.HealthCritical)

	// Check in warning state becomes healthy after check passes
	_, _, err := exec(nomadClient, allocs,
		[]string{"/bin/sh", "-c", "touch ${NOMAD_TASK_DIR}/alive-2b"})
	require.NoError(err)
	requireStatus(require, consulClient, "task-service-2", capi.HealthPassing)

	// Job update: verify checks are re-registered in Consul
	allocs = e2eutil.RegisterAndWaitForAllocs(f.T(),
		nomadClient, "consul/input/checks_task_update.nomad", jobId)
	require.Equal(1, len(allocs))
	requireStatus(require, consulClient, "task-service-1", capi.HealthPassing)
	requireStatus(require, consulClient, "task-service-2", capi.HealthPassing)
	requireStatus(require, consulClient, "task-service-3", capi.HealthCritical)

	// Verify we don't have any linger script checks running on the client
	out, _, err := exec(nomadClient, allocs, []string{"pgrep", "sleep"})
	require.NoError(err)
	running := strings.Split(strings.TrimSpace(out.String()), "\n")
	require.LessOrEqual(len(running), 2) // task itself + 1 check == 2

	// Clean job stop: verify that checks were deregistered in Consul
	nomadClient.Jobs().Deregister(jobId, false, nil) // nomad job stop
	requireDeregistered(require, consulClient, "task-service-1")
	requireDeregistered(require, consulClient, "task-service-2")
	requireDeregistered(require, consulClient, "task-service-3")

	// Restore for next test
	allocs = e2eutil.RegisterAndWaitForAllocs(f.T(),
		nomadClient, "consul/input/checks_task.nomad", jobId)
	require.Equal(2, len(allocs))
	requireStatus(require, consulClient, "task-service-1", capi.HealthPassing)
	requireStatus(require, consulClient, "task-service-2", capi.HealthWarning)
	requireStatus(require, consulClient, "task-service-3", capi.HealthCritical)

	// Crash a task: verify that checks become healthy again
	_, _, err = exec(nomadClient, allocs, []string{"pkill", "sleep"})
	if err != nil && err.Error() != "plugin is shut down" {
		require.FailNow("unexpected error: %v", err)
	}
	requireStatus(require, consulClient, "task-service-1", capi.HealthPassing)
	requireStatus(require, consulClient, "task-service-2", capi.HealthWarning)
	requireStatus(require, consulClient, "task-service-3", capi.HealthCritical)

	// TODO(tgross) ...
	// Restart client: verify that checks are re-registered
}

func (tc *ScriptChecksE2ETest) AfterEach(f *framework.F) {
	nomadClient := tc.Nomad()
	jobs := nomadClient.Jobs()
	// Stop all jobs in test
	for _, id := range tc.jobIds {
		jobs.Deregister(id, true, nil)
	}
	// Garbage collect
	nomadClient.System().GarbageCollect()
}

func exec(client *api.Client, allocs []*api.AllocationListStub, command []string) (bytes.Buffer, bytes.Buffer, error) {
	ctx, cancelFn := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelFn()

	// we're getting a list of from the registration call here but
	// one of them might be stopped or stopping, which will return
	// an error if we try to exec into it.
	var alloc *api.Allocation
	for _, stub := range allocs {
		if stub.DesiredStatus == "run" {
			alloc = &api.Allocation{
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
		make(chan api.TerminalSize), nil)
	return stdout, stderr, err
}

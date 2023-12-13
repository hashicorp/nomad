// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package remotetasks

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/jobspec2"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// ECS Task Statuses (currently unused statuses commented out to
	// appease linter)
	//ecsTaskStatusDeactivating   = "DEACTIVATING"
	//ecsTaskStatusStopping       = "STOPPING"
	//ecsTaskStatusDeprovisioning = "DEPROVISIONING"
	ecsTaskStatusStopped = "STOPPED"
	ecsTaskStatusRunning = "RUNNING"
)

type RemoteTasksTest struct {
	framework.TC
	jobIDs []string
}

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "RemoteTasks",
		CanRunLocal: true,
		Cases: []framework.TestCase{
			new(RemoteTasksTest),
		},
	})
}

func (tc *RemoteTasksTest) BeforeAll(f *framework.F) {
	e2eutil.WaitForLeader(f.T(), tc.Nomad())
	e2eutil.WaitForNodesReady(f.T(), tc.Nomad(), 2)
}

func (tc *RemoteTasksTest) AfterEach(f *framework.F) {
	nomadClient := tc.Nomad()

	// Mark all nodes eligible
	nodesAPI := tc.Nomad().Nodes()
	nodes, _, _ := nodesAPI.List(nil)
	for _, node := range nodes {
		nodesAPI.ToggleEligibility(node.ID, true, nil)
	}

	jobs := nomadClient.Jobs()
	// Stop all jobs in test
	for _, id := range tc.jobIDs {
		jobs.Deregister(id, true, nil)
	}
	tc.jobIDs = []string{}

	// Garbage collect
	nomadClient.System().GarbageCollect()
}

// TestECSJob asserts an ECS job may be started and is cleaned up when stopped.
func (tc *RemoteTasksTest) TestECSJob(f *framework.F) {
	t := f.T()

	ecsClient := ecsOrSkip(t, tc.Nomad())

	jobID := "ecsjob-" + uuid.Generate()[0:8]
	tc.jobIDs = append(tc.jobIDs, jobID)
	_, allocs := registerECSJobs(t, tc.Nomad(), jobID)
	require.Len(t, allocs, 1)
	allocID := allocs[0].ID
	e2eutil.WaitForAllocsRunning(t, tc.Nomad(), []string{allocID})

	// We need to go from Allocation -> ECS ARN, so grab the updated
	// allocation's task state.
	arn := arnForAlloc(t, tc.Nomad().Allocations(), allocID)

	// Use ARN to lookup status of ECS task in AWS
	ensureECSRunning(t, ecsClient, arn)

	t.Logf("Task %s is running!", arn)

	// Stop the job
	e2eutil.WaitForJobStopped(t, tc.Nomad(), jobID)

	// Ensure it is stopped in ECS
	input := ecs.DescribeTasksInput{
		Cluster: aws.String("nomad-rtd-e2e"),
		Tasks:   []*string{aws.String(arn)},
	}
	testutil.WaitForResult(func() (bool, error) {
		resp, err := ecsClient.DescribeTasks(&input)
		if err != nil {
			return false, err
		}
		status := *resp.Tasks[0].LastStatus
		return status == ecsTaskStatusStopped, fmt.Errorf("ecs task is not stopped: %s", status)
	}, func(err error) {
		t.Fatalf("error retrieving ecs task status: %v", err)
	})
}

// TestECSDrain asserts an ECS job may be started, drained from one node, and
// is managed by a new node without stopping and restarting the remote task.
func (tc *RemoteTasksTest) TestECSDrain(f *framework.F) {
	t := f.T()

	ecsClient := ecsOrSkip(t, tc.Nomad())

	jobID := "ecsjob-" + uuid.Generate()[0:8]
	tc.jobIDs = append(tc.jobIDs, jobID)
	_, allocs := registerECSJobs(t, tc.Nomad(), jobID)
	require.Len(t, allocs, 1)
	origNode := allocs[0].NodeID
	origAlloc := allocs[0].ID
	e2eutil.WaitForAllocsRunning(t, tc.Nomad(), []string{origAlloc})

	arn := arnForAlloc(t, tc.Nomad().Allocations(), origAlloc)
	ensureECSRunning(t, ecsClient, arn)

	t.Logf("Task %s is running! Now to drain the node.", arn)

	// Drain the node
	_, err := tc.Nomad().Nodes().UpdateDrain(
		origNode,
		&api.DrainSpec{Deadline: 30 * time.Second},
		false,
		nil,
	)
	require.NoError(t, err, "error draining original node")

	// Wait for new alloc to be running
	var newAlloc *api.AllocationListStub
	qopts := &api.QueryOptions{}
	testutil.WaitForResult(func() (bool, error) {
		allocs, resp, err := tc.Nomad().Jobs().Allocations(jobID, false, qopts)
		if err != nil {
			return false, fmt.Errorf("error retrieving allocations for job: %w", err)
		}

		qopts.WaitIndex = resp.LastIndex

		if len(allocs) > 2 {
			return false, fmt.Errorf("expected 1 or 2 allocs but found %d", len(allocs))
		}

		for _, alloc := range allocs {
			if alloc.ID == origAlloc {
				// This is the old alloc, skip it
				continue
			}

			newAlloc = alloc

			if newAlloc.ClientStatus == "running" {
				break
			}
		}

		if newAlloc == nil {
			return false, fmt.Errorf("no new alloc found")
		}
		if newAlloc.ClientStatus != "running" {
			return false, fmt.Errorf("expected new alloc (%s) to be running but found: %s",
				newAlloc.ID, newAlloc.ClientStatus)
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("error waiting for new alloc to be running: %v", err)
	})

	// Make sure the ARN hasn't changed by looking up the new alloc's ARN
	newARN := arnForAlloc(t, tc.Nomad().Allocations(), newAlloc.ID)

	assert.Equal(t, arn, newARN, "unexpected new ARN")
}

// TestECSDeployment asserts a new ECS task is started when an ECS job is
// deployed.
func (tc *RemoteTasksTest) TestECSDeployment(f *framework.F) {
	t := f.T()

	ecsClient := ecsOrSkip(t, tc.Nomad())

	jobID := "ecsjob-" + uuid.Generate()[0:8]
	tc.jobIDs = append(tc.jobIDs, jobID)
	job, origAllocs := registerECSJobs(t, tc.Nomad(), jobID)
	require.Len(t, origAllocs, 1)
	origAllocID := origAllocs[0].ID
	e2eutil.WaitForAllocsRunning(t, tc.Nomad(), []string{origAllocID})

	// We need to go from Allocation -> ECS ARN, so grab the updated
	// allocation's task state.
	origARN := arnForAlloc(t, tc.Nomad().Allocations(), origAllocID)

	// Use ARN to lookup status of ECS task in AWS
	ensureECSRunning(t, ecsClient, origARN)

	t.Logf("Task %s is running! Updating...", origARN)

	// Force a deployment by updating meta
	job.Meta = map[string]string{
		"updated": time.Now().Format(time.RFC3339Nano),
	}

	// Register updated job
	resp, _, err := tc.Nomad().Jobs().Register(job, nil)
	require.NoError(t, err, "error registering updated job")
	require.NotEmpty(t, resp.EvalID, "no eval id created when registering updated job")

	// Wait for new alloc to be running
	var newAlloc *api.AllocationListStub
	testutil.WaitForResult(func() (bool, error) {
		allocs, _, err := tc.Nomad().Jobs().Allocations(jobID, false, nil)
		if err != nil {
			return false, err
		}

		for _, a := range allocs {
			if a.ID == origAllocID {
				if a.ClientStatus == "complete" {
					// Original alloc stopped as expected!
					continue
				}

				// Original alloc is still running
				newAlloc = nil
				return false, fmt.Errorf("original alloc not yet terminal. "+
					"client status: %s; desired status: %s",
					a.ClientStatus, a.DesiredStatus)
			}

			if a.ClientStatus != "running" {
				return false, fmt.Errorf("new alloc is not running: %s", a.ClientStatus)
			}

			if newAlloc != nil {
				return false, fmt.Errorf("found 2 replacement allocs: %s and %s",
					a.ID, newAlloc.ID)
			}

			newAlloc = a
		}

		return newAlloc != nil, fmt.Errorf("no new alloc found for updated job")
	}, func(err error) {
		require.NoError(t, err, "error waiting for updated alloc")
	})

	newARN := arnForAlloc(t, tc.Nomad().Allocations(), newAlloc.ID)
	t.Logf("Task %s is updated!", newARN)
	require.NotEqual(t, origARN, newARN, "expected new ARN")

	// Ensure original ARN is stopped in ECS
	input := ecs.DescribeTasksInput{
		Cluster: aws.String("nomad-rtd-e2e"),
		Tasks:   []*string{aws.String(origARN)},
	}
	testutil.WaitForResult(func() (bool, error) {
		resp, err := ecsClient.DescribeTasks(&input)
		if err != nil {
			return false, err
		}
		status := *resp.Tasks[0].LastStatus
		return status == ecsTaskStatusStopped, fmt.Errorf("original ecs task is not stopped: %s", status)
	}, func(err error) {
		t.Fatalf("error retrieving ecs task status for original ARN: %v", err)
	})
}

// ecsOrSkip returns an AWS ECS client or skips the test if ECS is unreachable
// by the test runner or the ECS remote task driver isn't healthy.
func ecsOrSkip(t *testing.T, nomadClient *api.Client) *ecs.ECS {
	awsSession := session.Must(session.NewSession())

	ecsClient := ecs.New(awsSession, aws.NewConfig().WithRegion("us-east-1"))

	_, err := ecsClient.ListClusters(&ecs.ListClustersInput{})
	if err != nil {
		t.Skipf("Skipping ECS Remote Task Driver Task. Error querying AWS ECS API: %v", err)
	}

	testutil.WaitForResult(func() (bool, error) {
		nodes, _, err := nomadClient.Nodes().List(nil)
		if err != nil {
			return false, fmt.Errorf("error retrieving node listing: %w", err)
		}

		notReady := 0
		notEligible := 0
		noECS := 0
		notHealthy := 0
		ready := 0
		for _, n := range nodes {
			if n.Status != "ready" {
				notReady++
				continue
			}
			if n.SchedulingEligibility != "eligible" {
				notEligible++
				continue
			}
			ecsDriver, ok := n.Drivers["ecs"]
			if !ok {
				noECS++
				continue
			}
			if !ecsDriver.Healthy {
				notHealthy++
				continue
			}
			ready++
		}

		return ready > 1, fmt.Errorf("expected 2 nodes with healthy ecs drivers but found: "+
			"not_ready=%d ineligible=%d no_driver=%d unhealthy=%d ok=%d",
			notReady, notEligible, noECS, notHealthy, ready)
	}, func(err error) {
		if err != nil {
			t.Skipf("Skipping Remote Task Driver tests due to: %v", err)
		}
	})

	return ecsClient
}

// arnForAlloc retrieves the ARN for a running allocation.
func arnForAlloc(t *testing.T, allocAPI *api.Allocations, allocID string) string {
	t.Logf("Retrieving ARN for alloc=%s", allocID)
	ecsState := struct {
		ARN string
	}{}
	testutil.WaitForResult(func() (bool, error) {
		alloc, _, err := allocAPI.Info(allocID, nil)
		if err != nil {
			return false, err
		}
		state := alloc.TaskStates["http-server"]
		if state == nil {
			return false, fmt.Errorf("no task state for http-server (%d task states)", len(alloc.TaskStates))
		}
		if state.TaskHandle == nil {
			return false, fmt.Errorf("no task handle for http-server")
		}
		if len(state.TaskHandle.DriverState) == 0 {
			return false, fmt.Errorf("no driver state for task handle")
		}
		if err := base.MsgPackDecode(state.TaskHandle.DriverState, &ecsState); err != nil {
			return false, fmt.Errorf("error decoding driver state: %w", err)
		}
		if ecsState.ARN == "" {
			return false, fmt.Errorf("ARN is empty despite DriverState being %d bytes", len(state.TaskHandle.DriverState))
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("error getting ARN: %v", err)
	})
	t.Logf("Retrieved ARN=%s for alloc=%s", ecsState.ARN, allocID)

	return ecsState.ARN
}

// ensureECSRunning asserts that the given ARN is a running ECS task.
func ensureECSRunning(t *testing.T, ecsClient *ecs.ECS, arn string) {
	t.Logf("Ensuring ARN=%s is running", arn)
	input := ecs.DescribeTasksInput{
		Cluster: aws.String("nomad-rtd-e2e"),
		Tasks:   []*string{aws.String(arn)},
	}
	testutil.WaitForResult(func() (bool, error) {
		resp, err := ecsClient.DescribeTasks(&input)
		if err != nil {
			return false, err
		}
		status := *resp.Tasks[0].LastStatus
		return status == ecsTaskStatusRunning, fmt.Errorf("ecs task is not running: %s", status)
	}, func(err error) {
		t.Fatalf("error retrieving ecs task status: %v", err)
	})
	t.Logf("ARN=%s is running", arn)
}

// registerECSJobs registers an ECS job and returns it and its allocation
// stubs.
func registerECSJobs(t *testing.T, nomadClient *api.Client, jobID string) (*api.Job, []*api.AllocationListStub) {
	const (
		jobPath = "remotetasks/input/ecs.nomad"
		varPath = "remotetasks/input/ecs.vars"
	)

	jobBytes, err := os.ReadFile(jobPath)
	require.NoError(t, err, "error reading job file")

	job, err := jobspec2.ParseWithConfig(&jobspec2.ParseConfig{
		Path:     jobPath,
		Body:     jobBytes,
		VarFiles: []string{varPath},
		Strict:   true,
	})
	require.NoErrorf(t, err, "error parsing jobspec from %s with var file %s", jobPath, varPath)

	job.ID = &jobID
	job.Name = &jobID

	// Register job
	resp, _, err := nomadClient.Jobs().Register(job, nil)
	require.NoError(t, err, "error registering job")
	require.NotEmpty(t, resp.EvalID, "no eval id created when registering job")

	var allocs []*api.AllocationListStub
	testutil.WaitForResult(func() (bool, error) {
		allocs, _, err = nomadClient.Jobs().Allocations(jobID, false, nil)
		if err != nil {
			return false, err
		}
		return len(allocs) > 0, fmt.Errorf("no allocs found")
	}, func(err error) {
		require.NoErrorf(t, err, "error retrieving allocations for %s", jobID)
	})
	return job, allocs
}

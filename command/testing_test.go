package command

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
)

func testServer(t *testing.T, runClient bool, cb func(*agent.Config)) (*agent.TestAgent, *api.Client, string) {
	// Make a new test server
	a := agent.NewTestAgent(t, t.Name(), func(config *agent.Config) {
		config.Client.Enabled = runClient

		if cb != nil {
			cb(config)
		}
	})
	t.Cleanup(func() { _ = a.Shutdown() })

	c := a.Client()
	return a, c, a.HTTPAddr()
}

// testClient starts a new test client, blocks until it joins, and performs
// cleanup after the test is complete.
func testClient(t *testing.T, name string, cb func(*agent.Config)) (*agent.TestAgent, *api.Client, string) {
	t.Logf("Starting client agent %s", name)
	a := agent.NewTestAgent(t, name, func(config *agent.Config) {
		if cb != nil {
			cb(config)
		}
	})
	t.Cleanup(func() { _ = a.Shutdown() })

	c := a.Client()
	t.Logf("Waiting for client %s to join server(s) %s", name, a.GetConfig().Client.Servers)
	testutil.WaitForClient(t, a.Agent.RPC, a.Agent.Client().NodeID(), a.Agent.Client().Region())

	return a, c, a.HTTPAddr()
}

func testJob(jobID string) *api.Job {
	task := api.NewTask("task1", "mock_driver").
		SetConfig("kill_after", "1s").
		SetConfig("run_for", "5s").
		SetConfig("exit_code", 0).
		Require(&api.Resources{
			MemoryMB: pointer.Of(256),
			CPU:      pointer.Of(100),
		}).
		SetLogConfig(&api.LogConfig{
			MaxFiles:      pointer.Of(1),
			MaxFileSizeMB: pointer.Of(2),
		})

	group := api.NewTaskGroup("group1", 1).
		AddTask(task).
		RequireDisk(&api.EphemeralDisk{
			SizeMB: pointer.Of(20),
		})

	job := api.NewBatchJob(jobID, jobID, "global", 1).
		AddDatacenter("dc1").
		AddTaskGroup(group)

	return job
}

func testNomadServiceJob(jobID string) *api.Job {
	j := testJob(jobID)
	j.TaskGroups[0].Services = []*api.Service{{
		Name:        "service1",
		PortLabel:   "1000",
		AddressMode: "",
		Address:     "127.0.0.1",
		Checks: []api.ServiceCheck{{
			Name:     "check1",
			Type:     "http",
			Path:     "/",
			Interval: 1 * time.Second,
			Timeout:  1 * time.Second,
		}},
		Provider: "nomad",
	}}
	return j
}

func testMultiRegionJob(jobID, region, datacenter string) *api.Job {
	task := api.NewTask("task1", "mock_driver").
		SetConfig("kill_after", "10s").
		SetConfig("run_for", "15s").
		SetConfig("exit_code", 0).
		Require(&api.Resources{
			MemoryMB: pointer.Of(256),
			CPU:      pointer.Of(100),
		}).
		SetLogConfig(&api.LogConfig{
			MaxFiles:      pointer.Of(1),
			MaxFileSizeMB: pointer.Of(2),
		})

	group := api.NewTaskGroup("group1", 1).
		AddTask(task).
		RequireDisk(&api.EphemeralDisk{
			SizeMB: pointer.Of(20),
		})

	job := api.NewServiceJob(jobID, jobID, region, 1).AddDatacenter(datacenter).AddTaskGroup(group)
	job.Region = nil
	job.Multiregion = &api.Multiregion{
		Regions: []*api.MultiregionRegion{
			{
				Name:        "east",
				Datacenters: []string{"east-1"},
			},
			{
				Name:        "west",
				Datacenters: []string{"west-1"},
			},
		},
	}

	return job
}

func waitForNodes(t *testing.T, client *api.Client) {
	testutil.WaitForResult(func() (bool, error) {
		nodes, _, err := client.Nodes().List(nil)
		if err != nil {
			return false, err
		}
		for _, node := range nodes {
			if _, ok := node.Drivers["mock_driver"]; ok &&
				node.Status == structs.NodeStatusReady {
				return true, nil
			}
		}
		return false, fmt.Errorf("no ready nodes")
	}, func(err error) {
		must.NoError(t, err)
	})
}

func waitForAllocRunning(t *testing.T, client *api.Client, allocID string) {
	testutil.WaitForResult(func() (bool, error) {
		alloc, _, err := client.Allocations().Info(allocID, nil)
		if err != nil {
			return false, err
		}
		if alloc.ClientStatus == api.AllocClientStatusRunning {
			return true, nil
		}
		return false, fmt.Errorf("alloc status: %s", alloc.ClientStatus)
	}, func(err error) {
		t.Fatalf("timed out waiting for alloc to be running: %v", err)
	})
}

func waitForCheckStatus(t *testing.T, client *api.Client, allocID, status string) {
	testutil.WaitForResult(func() (bool, error) {
		results, err := client.Allocations().Checks(allocID, nil)
		if err != nil {
			return false, err
		}

		// pick a check, any check will do
		for _, check := range results {
			if check.Status == status {
				return true, nil
			}
		}

		return false, fmt.Errorf("no check with status: %s", status)
	}, func(err error) {
		t.Fatalf("timed out waiting for alloc to be running: %v", err)
	})
}

func getAllocFromJob(t *testing.T, client *api.Client, jobID string) string {
	var allocID string
	if allocations, _, err := client.Jobs().Allocations(jobID, false, nil); err == nil {
		if len(allocations) > 0 {
			allocID = allocations[0].ID
		}
	}
	must.NotEq(t, "", allocID, must.Sprint("expected to find an evaluation after running job", jobID))
	return allocID
}

func stopTestAgent(a *agent.TestAgent) {
	_ = a.Shutdown()
}

func getTempFile(t *testing.T, name string) (string, func()) {
	f, err := os.CreateTemp("", name)
	must.NoError(t, err)
	must.NoError(t, f.Close())
	return f.Name(), func() {
		_ = os.Remove(f.Name())
	}
}

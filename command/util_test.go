package command

import (
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/testutil"
)

func testServer(t *testing.T, runClient bool, cb func(*agent.Config)) (*agent.TestAgent, *api.Client, string) {
	// Make a new test server
	a := agent.NewTestAgent(t, t.Name(), func(config *agent.Config) {
		config.Client.Enabled = runClient

		if cb != nil {
			cb(config)
		}
	})
	t.Cleanup(func() { a.Shutdown() })

	c := a.Client()
	return a, c, a.HTTPAddr()
}

// testClient starts a new test client, blocks until it joins, and performs
// cleanup after the test is complete.
func testClient(t *testing.T, name string, cb func(*agent.Config)) (*agent.TestAgent, *api.Client, string) {
	t.Logf("[TEST] Starting client agent %s", name)
	a := agent.NewTestAgent(t, name, func(config *agent.Config) {
		if cb != nil {
			cb(config)
		}
	})
	t.Cleanup(func() { a.Shutdown() })

	c := a.Client()
	t.Logf("[TEST] Waiting for client %s to join server(s) %s", name, a.GetConfig().Client.Servers)
	testutil.WaitForClient(t, a.Agent.RPC, a.Agent.Client().NodeID(), a.Agent.Client().Region())

	return a, c, a.HTTPAddr()
}

func testJob(jobID string) *api.Job {
	task := api.NewTask("task1", "mock_driver").
		SetConfig("kill_after", "1s").
		SetConfig("run_for", "5s").
		SetConfig("exit_code", 0).
		Require(&api.Resources{
			MemoryMB: helper.IntToPtr(256),
			CPU:      helper.IntToPtr(100),
		}).
		SetLogConfig(&api.LogConfig{
			MaxFiles:      helper.IntToPtr(1),
			MaxFileSizeMB: helper.IntToPtr(2),
		})

	group := api.NewTaskGroup("group1", 1).
		AddTask(task).
		RequireDisk(&api.EphemeralDisk{
			SizeMB: helper.IntToPtr(20),
		})

	job := api.NewBatchJob(jobID, jobID, "global", 1).
		AddDatacenter("dc1").
		AddTaskGroup(group)

	return job
}

func testMultiRegionJob(jobID, region, datacenter string) *api.Job {
	task := api.NewTask("task1", "mock_driver").
		SetConfig("kill_after", "10s").
		SetConfig("run_for", "15s").
		SetConfig("exit_code", 0).
		Require(&api.Resources{
			MemoryMB: helper.IntToPtr(256),
			CPU:      helper.IntToPtr(100),
		}).
		SetLogConfig(&api.LogConfig{
			MaxFiles:      helper.IntToPtr(1),
			MaxFileSizeMB: helper.IntToPtr(2),
		})

	group := api.NewTaskGroup("group1", 1).
		AddTask(task).
		RequireDisk(&api.EphemeralDisk{
			SizeMB: helper.IntToPtr(20),
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

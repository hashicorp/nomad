package command

import (
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/helper"
)

func testServer(t *testing.T, runClient bool, cb func(*agent.Config)) (*agent.TestAgent, *api.Client, string) {
	// Make a new test server
	a := agent.NewTestAgent(t.Name(), func(config *agent.Config) {
		config.Client.Enabled = runClient

		if cb != nil {
			cb(config)
		}
	})

	c := a.Client()
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

	job := api.NewBatchJob(jobID, jobID, "region1", 1).
		AddDatacenter("dc1").
		AddTaskGroup(group)

	return job
}

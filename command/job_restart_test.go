package command

import (
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"

	"github.com/shoenig/test/must"
)

func TestJobRestartCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &JobRestartCommand{}
}

func TestJobRestartCommand_parseAndValidate(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name        string
		args        []string
		expectedErr string
		expectedCmd *JobRestartCommand
	}{
		{
			name:        "missing job",
			args:        []string{},
			expectedErr: "This command takes one or two argument",
		},
		{
			name:        "too many args",
			args:        []string{"one", "two", "three"},
			expectedErr: "This command takes one or two argument",
		},
		{
			name: "task from arg",
			args: []string{"my-job", "my-task"},
			expectedCmd: &JobRestartCommand{
				jobID:     "my-job",
				task:      "my-task",
				batchSize: 1,
			},
		},
		{
			name: "task flag takes precedence over task arg",
			args: []string{"-task", "your-task", "my-job", "my-task"},
			expectedCmd: &JobRestartCommand{
				jobID:     "my-job",
				task:      "your-task",
				batchSize: 1,
			},
		},
		{
			name: "all tasks",
			args: []string{"-all-tasks", "my-job"},
			expectedCmd: &JobRestartCommand{
				jobID:     "my-job",
				allTasks:  true,
				batchSize: 1,
			},
		},
		{
			name:        "all tasks conflicts with task flag",
			args:        []string{"-all-tasks", "-task", "my-task", "my-job"},
			expectedErr: "The -all-tasks option cannot be used when a task is defined",
		},
		{
			name:        "all tasks conflicts with task arg",
			args:        []string{"-all-tasks", "my-job", "my-task"},
			expectedErr: "The -all-tasks option cannot be used when a task is defined",
		},
		{
			name: "batch size as number",
			args: []string{"-batch", "10", "my-job"},
			expectedCmd: &JobRestartCommand{
				jobID:     "my-job",
				batchSize: 10,
			},
		},
		{
			name: "batch size as percentage",
			args: []string{"-batch", "10%", "my-job"},
			expectedCmd: &JobRestartCommand{
				jobID:            "my-job",
				batchSize:        10,
				batchSizePercent: true,
			},
		},
		{
			name: "batch size all",
			args: []string{"-batch", "all", "my-job"},
			expectedCmd: &JobRestartCommand{
				jobID:        "my-job",
				batchSizeAll: true,
			},
		},
		{
			name:        "batch size not valid",
			args:        []string{"-batch", "not-valid", "my-job"},
			expectedErr: "Invalid -batch value",
		},
		{
			name:        "batch size decimal",
			args:        []string{"-batch", "1.5", "my-job"},
			expectedErr: "Invalid -batch value",
		},
		{
			name:        "batch size zero",
			args:        []string{"-batch", "0", "my-job"},
			expectedErr: "Invalid -batch value",
		},
		{
			name:        "batch size decimal percent",
			args:        []string{"-batch", "1.5%", "my-job"},
			expectedErr: "Invalid -batch value",
		},
		{
			name:        "batch size zero percentage",
			args:        []string{"-batch", "0%", "my-job"},
			expectedErr: "Invalid -batch value",
		},
		{
			name:        "batch size with multiple numbers and percentages",
			args:        []string{"-batch", "15%10%", "my-job"},
			expectedErr: "Invalid -batch value",
		},
		{
			name:        "batch wait ask",
			args:        []string{"-wait", "ask", "my-job"},
			expectedErr: "terminal is not interactive", // Can't test non-interactive.
		},
		{
			name: "batch wait duration",
			args: []string{"-wait", "10s", "my-job"},
			expectedCmd: &JobRestartCommand{
				jobID:     "my-job",
				batchSize: 1,
				batchWait: 10 * time.Second,
			},
		},
		{
			name:        "batch wait invalid",
			args:        []string{"-wait", "10", "my-job"},
			expectedErr: "Invalid -wait value",
		},
		{
			name: "fail on error",
			args: []string{"-fail-on-error", "my-job"},
			expectedCmd: &JobRestartCommand{
				jobID:       "my-job",
				batchSize:   1,
				failOnError: true,
			},
		},
		{
			name: "no shutdown delay",
			args: []string{"-no-shutdown-delay", "my-job"},
			expectedCmd: &JobRestartCommand{
				jobID:           "my-job",
				batchSize:       1,
				noShutdownDelay: true,
			},
		},
		{
			name: "reschedule",
			args: []string{"-reschedule", "my-job"},
			expectedCmd: &JobRestartCommand{
				jobID:      "my-job",
				batchSize:  1,
				reschedule: true,
			},
		},
		{
			name:        "reschedule conflicts with task flag",
			args:        []string{"-reschedule", "-task", "my-task", "my-job"},
			expectedErr: "The -reschedule option cannot be used when a task is defined",
		},
		{
			name:        "reschedule conflicts with task arg",
			args:        []string{"-reschedule", "my-job", "my-task"},
			expectedErr: "The -reschedule option cannot be used when a task is defined",
		},
		{
			name: "verbose",
			args: []string{"-verbose", "my-job"},
			expectedCmd: &JobRestartCommand{
				jobID:     "my-job",
				batchSize: 1,
				verbose:   true,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &JobRestartCommand{}
			err := cmd.parseAndValidate(tc.args)

			if tc.expectedErr != "" {
				must.ErrorContains(t, err, tc.expectedErr)
			} else {
				must.NoError(t, err)
				must.Eq(t, tc.expectedCmd, cmd)
			}
		})
	}
}

func TestJobRestartCommand_fails(t *testing.T) {
	ci.Parallel(t)

	ui := cli.NewMockUi()
	cmd := &JobRestartCommand{Meta: Meta{Ui: ui}}

	// Start client and server and wait for node to be ready.
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	waitForNodes(t, client)

	// Register test job.
	jobID := "test_job_restart_cmd"
	job := testJob(jobID)
	resp, _, err := client.Jobs().Register(job, nil)
	must.NoError(t, err)

	code := waitForSuccess(ui, client, fullId, t, resp.EvalID)
	must.Zero(t, code)

	testCases := []struct {
		name         string
		args         []string
		expectedErr  string
		expectedCode int
	}{
		{
			name:         "connection failure",
			args:         []string{"-address=nope", "example"},
			expectedErr:  "Error listing jobs",
			expectedCode: 1,
		},
		{
			name:         "invalid job",
			args:         []string{"-address", url, "not-valid"},
			expectedErr:  `No job(s) with prefix or id "not-valid" found`,
			expectedCode: 1,
		},
		{
			name:         "invalid task arg",
			args:         []string{"-address", url, jobID, "not-valid"},
			expectedErr:  `No task named "not-valid" found `,
			expectedCode: 1,
		},
		{
			name:         "invalid task flag",
			args:         []string{"-address", url, "-task", "not-valid", jobID},
			expectedErr:  `No task named "not-valid" found `,
			expectedCode: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			code := cmd.Run(tc.args)
			must.Eq(t, tc.expectedCode, code)

			if tc.expectedErr != "" {
				out := ui.ErrorWriter.String()
				must.StrContains(t, out, tc.expectedErr)
			}

			ui.ErrorWriter.Reset()
		})
	}
}

func TestJobRestartCommand_success(t *testing.T) {
	ci.Parallel(t)

	// Start client and server and wait for node to be ready.
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	waitForNodes(t, client)

	// Register test job with several task, groups, and allocations.
	prestartTask := api.NewTask("prestart", "mock_driver").
		SetConfig("run_for", "100ms").
		SetConfig("exit_code", 0).
		SetLifecycle(&api.TaskLifecycle{
			Hook:    api.TaskLifecycleHookPrestart,
			Sidecar: false,
		})
	sidecarTask := api.NewTask("sidecar", "mock_driver").
		SetConfig("run_for", "1m").
		SetConfig("exit_code", 0).
		SetLifecycle(&api.TaskLifecycle{
			Hook:    api.TaskLifecycleHookPoststart,
			Sidecar: true,
		})
	mainTask := api.NewTask("main", "mock_driver").
		SetConfig("run_for", "1m").
		SetConfig("exit_code", 0)

	registerJob := func(t *testing.T, ui cli.Ui, jobID string) func(*testing.T) {
		job := api.NewServiceJob(jobID, jobID, "global", 1).
			AddDatacenter("dc1").
			AddTaskGroup(
				api.NewTaskGroup("single_task", 3).
					AddTask(mainTask),
			).
			AddTaskGroup(
				api.NewTaskGroup("multiple_tasks", 2).
					AddTask(prestartTask).
					AddTask(sidecarTask).
					AddTask(mainTask),
			)

		resp, _, err := client.Jobs().Register(job, nil)
		must.NoError(t, err)

		code := waitForSuccess(ui, client, fullId, t, resp.EvalID)
		must.Zero(t, code)

		// Wait for allocs to be running.
		allocs, _, err := client.Jobs().Allocations(jobID, true, nil)
		must.NoError(t, err)
		for _, alloc := range allocs {
			waitForAllocRunning(t, client, alloc.ID)
		}

		return func(t *testing.T) {
			_, _, err := client.Jobs().Deregister(jobID, true, nil)
			must.NoError(t, err)

			// Wait for allocs to be complete.
			allocs, _, err := client.Jobs().Allocations(jobID, true, nil)
			must.NoError(t, err)
			for _, alloc := range allocs {
				waitForAllocStatus(t, client, alloc.ID, api.AllocClientStatusComplete)
			}
		}
	}

	testCases := []struct {
		name       string
		args       []string // Pass task using the -task flag. Job is added
		validateFn func(*testing.T, []*api.Allocation, string)
	}{
		{
			name: "restart only running tasks by default",
			args: []string{"-batch", "all"},
			validateFn: func(t *testing.T, allocs []*api.Allocation, out string) {
				restarted := waitTasksRestarted(t, client, allocs, map[string]bool{
					"prestart": false,
					"sidecar":  true,
					"main":     true,
				})

				// Check that allocations were not batched.
				batches := getRestartBatches(restarted, "main")
				must.Len(t, 5, batches[0])
				must.StrNotContains(t, out, "Restarting 1st batch")
				must.StrNotContains(t, out, "restarting the next batch")
			},
		},
		{
			name: "restart specific task",
			args: []string{"-batch", "all", "-task", "main"},
			validateFn: func(t *testing.T, allocs []*api.Allocation, out string) {
				restarted := waitTasksRestarted(t, client, allocs, map[string]bool{
					"prestart": false,
					"sidecar":  false,
					"main":     true,
				})

				// Check that allocations were not batched.
				batches := getRestartBatches(restarted, "main")
				must.Len(t, 5, batches[0])
				must.StrNotContains(t, out, "Restarting 1st batch")
				must.StrNotContains(t, out, "restarting the next batch")
			},
		},
		{
			name: "restart all tasks",
			args: []string{"-batch", "all", "-all-tasks"},
			validateFn: func(t *testing.T, allocs []*api.Allocation, out string) {
				restarted := waitTasksRestarted(t, client, allocs, map[string]bool{
					"prestart": true,
					"sidecar":  true,
					"main":     true,
				})

				// Check that allocations were not batched.
				batches := getRestartBatches(restarted, "main")
				must.Len(t, 5, batches[0])
				must.StrNotContains(t, out, "Restarting 1st batch")
				must.StrNotContains(t, out, "restarting the next batch")
			},
		},
		{
			name: "restart in batches",
			args: []string{"-batch", "3", "-wait", "3s", "-task", "main"},
			validateFn: func(t *testing.T, allocs []*api.Allocation, out string) {
				restarted := waitTasksRestarted(t, client, allocs, map[string]bool{
					"prestart": false,
					"sidecar":  false,
					"main":     true,
				})

				// Check that allocations were properly batched.
				batches := getRestartBatches(restarted, "main")

				must.Len(t, 3, batches[0])
				must.StrContains(t, out, "Restarting 1st batch of 3 allocations")
				must.StrContains(t, out, "Waiting 3s before restarting the next batch")

				must.Len(t, 2, batches[1])
				must.StrContains(t, out, "Restarting 2nd batch of 3 allocations")

				// Check that batches waited the expected time.
				batch1Restart := batches[0][0].TaskStates["main"].LastRestart
				batch2Restart := batches[1][0].TaskStates["main"].LastRestart
				diff := batch2Restart.Sub(batch1Restart)
				must.Between(t, 3*time.Second, diff, 4*time.Second)
			},
		},
		{
			name: "restart in percent batch",
			args: []string{"-batch", "50%", "-wait", "3s", "-task", "main"},
			validateFn: func(t *testing.T, allocs []*api.Allocation, out string) {
				restarted := waitTasksRestarted(t, client, allocs, map[string]bool{
					"prestart": false,
					"sidecar":  false,
					"main":     true,
				})

				// Check that allocations were properly batched.
				batches := getRestartBatches(restarted, "main")

				must.Len(t, 2, batches[0])
				must.StrContains(t, out, "Restarting 1st batch of 2 allocations")
				must.StrContains(t, out, "Waiting 3s before restarting the next batch")

				must.Len(t, 2, batches[1])
				must.StrContains(t, out, "Restarting 2nd batch of 2 allocations")

				must.Len(t, 1, batches[2])
				must.StrContains(t, out, "Restarting 3rd batch of 2 allocations")

				// Check that batches waited the expected time.
				batch1Restart := batches[0][0].TaskStates["main"].LastRestart
				batch2Restart := batches[1][0].TaskStates["main"].LastRestart
				batch3Restart := batches[2][0].TaskStates["main"].LastRestart

				diff := batch2Restart.Sub(batch1Restart)
				must.Between(t, 3*time.Second, diff, 4*time.Second)

				diff = batch3Restart.Sub(batch2Restart)
				must.Between(t, 3*time.Second, diff, 4*time.Second)
			},
		},
		{
			name: "reschedule in batches",
			args: []string{"-reschedule", "-batch", "3"},
			validateFn: func(t *testing.T, allocs []*api.Allocation, out string) {
				waitAllocsRescheduled(t, client, allocs)

				// Check that allocations were properly batched.
				must.StrContains(t, out, "Restarting 1st batch of 3 allocations")
				must.StrContains(t, out, "Restarting 2nd batch of 3 allocations")
				must.StrNotContains(t, out, "Waiting")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ui := cli.NewMockUi()
			cmd := &JobRestartCommand{Meta: Meta{Ui: ui}}

			// Print command output to help with debugging when verbose or test
			// case fails.
			defer func() {
				t.Log(ui.OutputWriter.String())
				t.Log(ui.ErrorWriter.String())

				ui.OutputWriter.Reset()
				ui.ErrorWriter.Reset()
			}()

			// Register new job to prevent old state from interfering with each
			// test case.
			jobID := strings.ReplaceAll(tc.name, " ", "_")
			deregister := registerJob(t, ui, jobID)
			defer deregister(t)

			// Prepend server URL and append job ID to the command.
			args := []string{"-address", url}
			args = append(args, tc.args...)
			args = append(args, jobID)

			// Fetch allocations with full details so we can check their task
			// events.
			allocStubs, _, err := client.Jobs().Allocations(jobID, true, nil)
			must.NoError(t, err)

			allocs := make([]*api.Allocation, 0, len(allocStubs))
			for _, stub := range allocStubs {
				alloc, _, err := client.Allocations().Info(stub.ID, nil)
				must.NoError(t, err)
				allocs = append(allocs, alloc)
			}

			// Run job restart command.
			code := cmd.Run(args)
			must.Zero(t, code)

			// Run test case validation function and reset UI outputs.
			tc.validateFn(t, allocs, ui.OutputWriter.String())
		})
	}
}

func TestJobRestartCommand_reschedule_fail(t *testing.T) {
	ci.Parallel(t)

	ui := cli.NewMockUi()
	cmd := &JobRestartCommand{Meta: Meta{Ui: ui}}

	// Print command output to help with debugging when verbose or test
	// case fails.
	defer func() {
		t.Log(ui.OutputWriter.String())
		t.Log(ui.ErrorWriter.String())
	}()

	// Start client and server and wait for node to be ready.
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	waitForNodes(t, client)

	// Register test job with 3 allocs.
	jobID := "test_job_restart_reschedule_fail"
	job := testJob(jobID)
	job.TaskGroups[0].Count = pointer.Of(3)

	resp, _, err := client.Jobs().Register(job, nil)
	must.NoError(t, err)

	code := waitForSuccess(ui, client, fullId, t, resp.EvalID)
	must.Zero(t, code)
	ui.OutputWriter.Reset()

	// Wait for allocs to be running.
	allocs, _, err := client.Jobs().Allocations(jobID, true, nil)
	must.NoError(t, err)
	for _, alloc := range allocs {
		waitForAllocRunning(t, client, alloc.ID)
	}

	// Mark node as ineligible to prevent allocs from being replaced.
	nodeID := srv.Agent.Client().NodeID()
	client.Nodes().ToggleEligibility(nodeID, false, nil)

	// Run job restart command.
	code = cmd.Run([]string{
		"-address", url,
		"-batch", "2",
		"-reschedule",
		jobID,
	})
	must.One(t, code)
}

func waitTasksRestarted(t *testing.T, client *api.Client, allocs []*api.Allocation, restarts map[string]bool) []*api.Allocation {
	t.Helper()

	var newAllocs []*api.Allocation
	testutil.WaitForResult(func() (bool, error) {
		newAllocs = make([]*api.Allocation, 0, len(allocs))

		for _, alloc := range allocs {
			updated, _, err := client.Allocations().Info(alloc.ID, nil)
			if err != nil {
				return false, err
			}
			newAllocs = append(newAllocs, updated)

			for task, state := range updated.TaskStates {
				restarted := false
				for _, ev := range state.Events {
					if ev.Type == api.TaskRestartSignal {
						restarted = true
						break
					}
				}
				if restarted {
					if !restarts[task] {
						return false, fmt.Errorf("task %s in alloc %s not expected to restart", task, updated.ID)
					}
				} else {
					if restarts[task] {
						return false, fmt.Errorf("task %s in alloc %s expected to restart but didn't", task, updated.ID)
					}
				}
			}
		}
		return false, nil
	}, func(err error) {
		must.NoError(t, err)
	})

	return newAllocs
}

func waitAllocsRescheduled(t *testing.T, client *api.Client, allocs []*api.Allocation) []*api.Allocation {
	t.Helper()

	var newAllocs []*api.Allocation
	testutil.WaitForResult(func() (bool, error) {
		newAllocs = make([]*api.Allocation, 0, len(allocs))

		for _, alloc := range allocs {
			updated, _, err := client.Allocations().Info(alloc.ID, nil)
			if err != nil {
				return false, err
			}
			newAllocs = append(newAllocs, updated)

			if updated.NextAllocation == "" {
				return false, fmt.Errorf("alloc %s doesn't have replacement", updated.ID)
			}
		}
		return true, nil
	}, func(err error) {
		must.NoError(t, err)
	})

	return newAllocs
}

func getRestartBatches(allocs []*api.Allocation, task string) [][]*api.Allocation {
	batches := [][]*api.Allocation{}

	type allocRestart struct {
		alloc   *api.Allocation
		restart time.Time
	}

	restarts := make([]allocRestart, 0, len(allocs))
	for _, alloc := range allocs {
		restarts = append(restarts, allocRestart{
			alloc:   alloc,
			restart: alloc.TaskStates[task].LastRestart,
		})
	}

	sort.Slice(restarts, func(i, j int) bool {
		return restarts[i].restart.Before(restarts[j].restart)
	})

	prev := restarts[0].restart
	batch := []*api.Allocation{}
	for _, r := range restarts {
		if r.restart.Sub(prev) >= time.Second {
			prev = r.restart
			batches = append(batches, batch)
			batch = []*api.Allocation{}
		}
		batch = append(batch, r.alloc)
	}
	batches = append(batches, batch)

	return batches
}

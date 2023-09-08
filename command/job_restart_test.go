// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	neturl "net/url"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/hashicorp/go-set"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"

	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
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
			expectedErr: "This command takes one argument",
		},
		{
			name:        "too many args",
			args:        []string{"one", "two", "three"},
			expectedErr: "This command takes one argument",
		},
		{
			name: "tasks and groups",
			args: []string{
				"-task", "my-task-1", "-task", "my-task-2",
				"-group", "my-group-1", "-group", "my-group-2",
				"my-job",
			},
			expectedCmd: &JobRestartCommand{
				jobID:     "my-job",
				groups:    set.From([]string{"my-group-1", "my-group-2"}),
				tasks:     set.From([]string{"my-task-1", "my-task-2"}),
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
			name:        "all tasks conflicts with task",
			args:        []string{"-all-tasks", "-task", "my-task", "-yes", "my-job"},
			expectedErr: "The -all-tasks option cannot be used with -task",
		},
		{
			name: "batch size as number",
			args: []string{"-batch-size", "10", "my-job"},
			expectedCmd: &JobRestartCommand{
				jobID:     "my-job",
				batchSize: 10,
			},
		},
		{
			name: "batch size as percentage",
			args: []string{"-batch-size", "10%", "my-job"},
			expectedCmd: &JobRestartCommand{
				jobID:            "my-job",
				batchSize:        10,
				batchSizePercent: true,
			},
		},
		{
			name:        "batch size not valid",
			args:        []string{"-batch-size", "not-valid", "my-job"},
			expectedErr: "Invalid -batch-size value",
		},
		{
			name:        "batch size decimal not valid",
			args:        []string{"-batch-size", "1.5", "my-job"},
			expectedErr: "Invalid -batch-size value",
		},
		{
			name:        "batch size zero",
			args:        []string{"-batch-size", "0", "my-job"},
			expectedErr: "Invalid -batch-size value",
		},
		{
			name:        "batch size decimal percent not valid",
			args:        []string{"-batch-size", "1.5%", "my-job"},
			expectedErr: "Invalid -batch-size value",
		},
		{
			name:        "batch size zero percentage",
			args:        []string{"-batch-size", "0%", "my-job"},
			expectedErr: "Invalid -batch-size value",
		},
		{
			name:        "batch size with multiple numbers and percentages",
			args:        []string{"-batch-size", "15%10%", "my-job"},
			expectedErr: "Invalid -batch-size value",
		},
		{
			name:        "batch wait ask",
			args:        []string{"-batch-wait", "ask", "my-job"},
			expectedErr: "terminal is not interactive", // Can't test non-interactive.
		},
		{
			name: "batch wait duration",
			args: []string{"-batch-wait", "10s", "my-job"},
			expectedCmd: &JobRestartCommand{
				jobID:     "my-job",
				batchSize: 1,
				batchWait: 10 * time.Second,
			},
		},
		{
			name:        "batch wait invalid",
			args:        []string{"-batch-wait", "10", "my-job"},
			expectedErr: "Invalid -batch-wait value",
		},
		{
			name: "on error fail",
			args: []string{"-on-error", "fail", "my-job"},
			expectedCmd: &JobRestartCommand{
				jobID:     "my-job",
				batchSize: 1,
				onError:   jobRestartOnErrorFail,
			},
		},
		{
			name:        "on error invalid",
			args:        []string{"-on-error", "invalid", "my-job"},
			expectedErr: "Invalid -on-error value",
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
			name:        "reschedule conflicts with task",
			args:        []string{"-reschedule", "-task", "my-task", "-yes", "my-job"},
			expectedErr: "The -reschedule option cannot be used with -task",
		},
		{
			name: "verbose",
			args: []string{"-verbose", "my-job"},
			expectedCmd: &JobRestartCommand{
				jobID:     "my-job",
				batchSize: 1,
				verbose:   true,
				length:    fullId,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ui := &cli.ConcurrentUi{Ui: cli.NewMockUi()}
			meta := Meta{Ui: ui}

			// Set some default values if not defined in test case.
			if tc.expectedCmd != nil {
				tc.expectedCmd.Meta = meta

				if tc.expectedCmd.length == 0 {
					tc.expectedCmd.length = shortId
				}
				if tc.expectedCmd.groups == nil {
					tc.expectedCmd.groups = set.New[string](0)
				}
				if tc.expectedCmd.tasks == nil {
					tc.expectedCmd.tasks = set.New[string](0)
				}
				if tc.expectedCmd.onError == "" {
					tc.expectedCmd.onError = jobRestartOnErrorAsk
					tc.expectedCmd.autoYes = true
					tc.args = append([]string{"-yes"}, tc.args...)
				}
			}

			cmd := &JobRestartCommand{Meta: meta}
			code, err := cmd.parseAndValidate(tc.args)

			if tc.expectedErr != "" {
				must.NonZero(t, code)
				must.ErrorContains(t, err, tc.expectedErr)
			} else {
				must.NoError(t, err)
				must.Zero(t, code)
				must.Eq(t, tc.expectedCmd, cmd, must.Cmp(cmpopts.IgnoreFields(JobRestartCommand{}, "Meta", "Meta.Ui")))
			}
		})
	}
}

func TestJobRestartCommand_Run(t *testing.T) {
	ci.Parallel(t)

	// Create a job with multiple tasks, groups, and allocations.
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

	jobID := "test_job_restart_cmd"
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

	testCases := []struct {
		name         string
		args         []string // Job arg is added automatically.
		expectedCode int
		validateFn   func(*testing.T, *api.Client, []*api.AllocationListStub, string, string)
	}{
		{
			name: "restart only running tasks in all groups by default",
			args: []string{"-batch-size", "100%"},
			validateFn: func(t *testing.T, client *api.Client, allocs []*api.AllocationListStub, stdout string, stderr string) {
				restarted := waitTasksRestarted(t, client, allocs, map[string]map[string]bool{
					"single_task": {
						"main": true,
					},
					"multiple_tasks": {
						"prestart": false,
						"sidecar":  true,
						"main":     true,
					},
				})

				// Check that allocations restarted in a single batch.
				batches := getRestartBatches(restarted, []string{"single_task", "multiple_tasks"}, "main")
				must.Len(t, 5, batches[0])
				must.StrContains(t, stdout, "Restarting 1st batch")
				must.StrNotContains(t, stdout, "restarting the next batch")

			},
		},
		{
			name: "restart specific task in all groups",
			args: []string{"-batch-size", "100%", "-task", "main"},
			validateFn: func(t *testing.T, client *api.Client, allocs []*api.AllocationListStub, stdout string, stderr string) {
				restarted := waitTasksRestarted(t, client, allocs, map[string]map[string]bool{
					"single_task": {
						"main": true,
					},
					"multiple_tasks": {
						"prestart": false,
						"sidecar":  false,
						"main":     true,
					},
				})

				// Check that allocations restarted in a single batch.
				batches := getRestartBatches(restarted, []string{"single_task", "multiple_tasks"}, "main")
				must.Len(t, 5, batches[0])
				must.StrContains(t, stdout, "Restarting 1st batch")
				must.StrNotContains(t, stdout, "restarting the next batch")
			},
		},
		{
			name: "restart multiple tasks in all groups",
			args: []string{"-batch-size", "100%", "-task", "main", "-task", "sidecar"},
			validateFn: func(t *testing.T, client *api.Client, allocs []*api.AllocationListStub, stdout string, stderr string) {
				restarted := waitTasksRestarted(t, client, allocs, map[string]map[string]bool{
					"single_task": {
						"main": true,
					},
					"multiple_tasks": {
						"prestart": false,
						"sidecar":  true,
						"main":     true,
					},
				})

				// Check that allocations restarted in a single batch.
				batches := getRestartBatches(restarted, []string{"single_task", "multiple_tasks"}, "main")
				must.Len(t, 5, batches[0])
				must.StrContains(t, stdout, "Restarting 1st batch")
				must.StrNotContains(t, stdout, "restarting the next batch")
			},
		},
		{
			name: "restart all tasks in all groups",
			args: []string{"-batch-size", "100%", "-all-tasks"},
			validateFn: func(t *testing.T, client *api.Client, allocs []*api.AllocationListStub, stdout string, stderr string) {
				restarted := waitTasksRestarted(t, client, allocs, map[string]map[string]bool{
					"single_task": {
						"main": true,
					},
					"multiple_tasks": {
						"prestart": true,
						"sidecar":  true,
						"main":     true,
					},
				})

				// Check that allocations restarted in a single batch.
				batches := getRestartBatches(restarted, []string{"single_task", "multiple_tasks"}, "main")
				must.Len(t, 5, batches[0])
				must.StrContains(t, stdout, "Restarting 1st batch")
				must.StrNotContains(t, stdout, "restarting the next batch")
			},
		},
		{
			name: "restart running tasks in specific group",
			args: []string{"-batch-size", "100%", "-group", "single_task"},
			validateFn: func(t *testing.T, client *api.Client, allocs []*api.AllocationListStub, stdout string, stderr string) {
				restarted := waitTasksRestarted(t, client, allocs, map[string]map[string]bool{
					"single_task": {
						"main": true,
					},
					"multiple_tasks": {
						"prestart": false,
						"sidecar":  false,
						"main":     false,
					},
				})

				// Check that allocations restarted in a single batch.
				batches := getRestartBatches(restarted, []string{"single_task"}, "main")
				must.Len(t, 3, batches[0])
				must.StrContains(t, stdout, "Restarting 1st batch")
				must.StrNotContains(t, stdout, "restarting the next batch")

			},
		},
		{
			name: "restart specific task that is not running",
			args: []string{"-batch-size", "100%", "-task", "prestart"},
			validateFn: func(t *testing.T, client *api.Client, allocs []*api.AllocationListStub, stdout string, stderr string) {
				restarted := waitTasksRestarted(t, client, allocs, map[string]map[string]bool{
					"single_task": {
						"main": false,
					},
					"multiple_tasks": {
						"prestart": false,
						"sidecar":  false,
						"main":     false,
					},
				})

				// Check that allocations restarted in a single batch.
				batches := getRestartBatches(restarted, []string{"single_task"}, "main")
				must.Len(t, 3, batches[0])
				must.StrContains(t, stdout, "Restarting 1st batch")
				must.StrNotContains(t, stdout, "restarting the next batch")

				// Check that we have an error message.
				must.StrContains(t, stderr, "Task not running")
			},
			expectedCode: 1,
		},
		{
			name: "restart specific task in specific group",
			args: []string{"-batch-size", "100%", "-task", "main", "-group", "single_task"},
			validateFn: func(t *testing.T, client *api.Client, allocs []*api.AllocationListStub, stdout string, stderr string) {
				restarted := waitTasksRestarted(t, client, allocs, map[string]map[string]bool{
					"single_task": {
						"main": true,
					},
					"multiple_tasks": {
						"prestart": false,
						"sidecar":  false,
						"main":     false,
					},
				})

				// Check that allocations restarted in a single batch.
				batches := getRestartBatches(restarted, []string{"single_task"}, "main")
				must.Len(t, 3, batches[0])
				must.StrContains(t, stdout, "Restarting 1st batch")
				must.StrNotContains(t, stdout, "restarting the next batch")
			},
		},
		{
			name: "restart multiple tasks in specific group",
			args: []string{"-batch-size", "100%", "-task", "main", "-task", "sidecar", "-group", "multiple_tasks"},
			validateFn: func(t *testing.T, client *api.Client, allocs []*api.AllocationListStub, stdout string, stderr string) {
				restarted := waitTasksRestarted(t, client, allocs, map[string]map[string]bool{
					"single_task": {
						"main": false,
					},
					"multiple_tasks": {
						"prestart": false,
						"sidecar":  true,
						"main":     true,
					},
				})

				// Check that allocations restarted in a single batch.
				batches := getRestartBatches(restarted, []string{"multiple_tasks"}, "main")
				must.Len(t, 2, batches[0])
				must.StrContains(t, stdout, "Restarting 1st batch")
				must.StrNotContains(t, stdout, "restarting the next batch")
			},
		},
		{
			name: "restart all tasks in specific group",
			args: []string{"-batch-size", "100%", "-all-tasks", "-group", "multiple_tasks"},
			validateFn: func(t *testing.T, client *api.Client, allocs []*api.AllocationListStub, stdout string, stderr string) {
				restarted := waitTasksRestarted(t, client, allocs, map[string]map[string]bool{
					"single_task": {
						"main": false,
					},
					"multiple_tasks": {
						"prestart": true,
						"sidecar":  true,
						"main":     true,
					},
				})

				// Check that allocations restarted in a single batch.
				batches := getRestartBatches(restarted, []string{"multiple_tasks"}, "main")
				must.Len(t, 2, batches[0])
				must.StrContains(t, stdout, "Restarting 1st batch")
				must.StrNotContains(t, stdout, "restarting the next batch")
			},
		},
		{
			name: "restart in batches",
			args: []string{"-batch-size", "3", "-batch-wait", "3s", "-task", "main"},
			validateFn: func(t *testing.T, client *api.Client, allocs []*api.AllocationListStub, stdout string, stderr string) {
				restarted := waitTasksRestarted(t, client, allocs, map[string]map[string]bool{
					"single_task": {
						"main": true,
					},
					"multiple_tasks": {
						"prestart": false,
						"sidecar":  false,
						"main":     true,
					},
				})

				// Check that allocations were properly batched.
				batches := getRestartBatches(restarted, []string{"multiple_tasks", "single_task"}, "main")

				must.Len(t, 3, batches[0])
				must.StrContains(t, stdout, "Restarting 1st batch of 3 allocations")

				must.Len(t, 2, batches[1])
				must.StrContains(t, stdout, "Restarting 2nd batch of 2 allocations")

				// Check that we only waited between batches.
				waitMsgCount := strings.Count(stdout, "Waiting 3s before restarting the next batch")
				must.Eq(t, 1, waitMsgCount)

				// Check that batches waited the expected time.
				batch1Restart := batches[0][0].TaskStates["main"].LastRestart
				batch2Restart := batches[1][0].TaskStates["main"].LastRestart
				diff := batch2Restart.Sub(batch1Restart)
				must.Between(t, 3*time.Second, diff, 4*time.Second)
			},
		},
		{
			name: "restart in percent batch",
			args: []string{"-batch-size", "50%", "-batch-wait", "3s", "-task", "main"},
			validateFn: func(t *testing.T, client *api.Client, allocs []*api.AllocationListStub, stdout string, stderr string) {
				restarted := waitTasksRestarted(t, client, allocs, map[string]map[string]bool{
					"single_task": {
						"main": true,
					},
					"multiple_tasks": {
						"prestart": false,
						"sidecar":  false,
						"main":     true,
					},
				})

				// Check that allocations were properly batched.
				batches := getRestartBatches(restarted, []string{"multiple_tasks", "single_task"}, "main")

				must.Len(t, 3, batches[0])
				must.StrContains(t, stdout, "Restarting 1st batch of 3 allocations")

				must.Len(t, 2, batches[1])
				must.StrContains(t, stdout, "Restarting 2nd batch of 2 allocations")

				// Check that we only waited between batches.
				waitMsgCount := strings.Count(stdout, "Waiting 3s before restarting the next batch")
				must.Eq(t, 1, waitMsgCount)

				// Check that batches waited the expected time.
				batch1Restart := batches[0][0].TaskStates["main"].LastRestart
				batch2Restart := batches[1][0].TaskStates["main"].LastRestart
				diff := batch2Restart.Sub(batch1Restart)
				must.Between(t, 3*time.Second, diff, 4*time.Second)
			},
		},
		{
			name: "restart in batch ask with yes",
			args: []string{"-batch-size", "100%", "-batch-wait", "ask", "-yes", "-group", "single_task"},
			validateFn: func(t *testing.T, client *api.Client, allocs []*api.AllocationListStub, stdout string, stderr string) {
				restarted := waitTasksRestarted(t, client, allocs, map[string]map[string]bool{
					"single_task": {
						"main": true,
					},
					"multiple_tasks": {
						"prestart": false,
						"sidecar":  false,
						"main":     false,
					},
				})

				// Check that allocations restarted in a single batch.
				batches := getRestartBatches(restarted, []string{"single_task"}, "main")
				must.Len(t, 3, batches[0])
				must.StrContains(t, stdout, "Restarting 1st batch")
				must.StrNotContains(t, stdout, "restarting the next batch")
			},
		},
		{
			name: "reschedule in batches",
			args: []string{"-reschedule", "-batch-size", "3"},
			validateFn: func(t *testing.T, client *api.Client, allocs []*api.AllocationListStub, stdout string, stderr string) {
				// Expect all allocations were rescheduled.
				reschedules := map[string]bool{}
				for _, alloc := range allocs {
					reschedules[alloc.ID] = true
				}
				waitAllocsRescheduled(t, client, reschedules)

				// Check that allocations were properly batched.
				must.StrContains(t, stdout, "Restarting 1st batch of 3 allocations")
				must.StrContains(t, stdout, "Restarting 2nd batch of 2 allocations")
				must.StrNotContains(t, stdout, "Waiting")
			},
		},
		{
			name: "reschedule specific group",
			args: []string{"-reschedule", "-batch-size", "100%", "-group", "single_task"},
			validateFn: func(t *testing.T, client *api.Client, allocs []*api.AllocationListStub, stdout string, stderr string) {
				// Expect that only allocs for the single_task group were
				// rescheduled.
				reschedules := map[string]bool{}
				for _, alloc := range allocs {
					if alloc.TaskGroup == "single_task" {
						reschedules[alloc.ID] = true
					}
				}
				waitAllocsRescheduled(t, client, reschedules)

				// Check that allocations restarted in a single batch.
				must.StrContains(t, stdout, "Restarting 1st batch")
				must.StrNotContains(t, stdout, "restarting the next batch")
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Run each test case in parallel because they are fairly slow.
			ci.Parallel(t)

			// Initialize UI and command.
			ui := cli.NewMockUi()
			cmd := &JobRestartCommand{Meta: Meta{Ui: ui}}

			// Start client and server and wait for node to be ready.
			// User separate cluster for each test case so they can run in
			// parallel without affecting each other.
			srv, client, url := testServer(t, true, nil)
			defer srv.Shutdown()

			waitForNodes(t, client)

			// Register test job and wait for its allocs to be running.
			resp, _, err := client.Jobs().Register(job, nil)
			must.NoError(t, err)

			code := waitForSuccess(ui, client, fullId, t, resp.EvalID)
			must.Zero(t, code)

			allocStubs, _, err := client.Jobs().Allocations(jobID, true, nil)
			must.NoError(t, err)
			for _, alloc := range allocStubs {
				waitForAllocRunning(t, client, alloc.ID)
			}

			// Fetch allocations before the restart so we know which ones are
			// supposed to be affected in case the test reschedules allocs.
			allocStubs, _, err = client.Jobs().Allocations(jobID, true, nil)
			must.NoError(t, err)

			// Prepend server URL and append job ID to the test case command.
			args := []string{"-address", url, "-yes"}
			args = append(args, tc.args...)
			args = append(args, jobID)

			// Run job restart command.
			code = cmd.Run(args)
			must.Eq(t, code, tc.expectedCode)

			// Run test case validation function.
			if tc.validateFn != nil {
				tc.validateFn(t, client, allocStubs, ui.OutputWriter.String(), ui.ErrorWriter.String())
			}
		})
	}
}

func TestJobRestartCommand_jobPrefixAndNamespace(t *testing.T) {
	ci.Parallel(t)

	ui := cli.NewMockUi()

	// Start client and server and wait for node to be ready.
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	waitForNodes(t, client)

	// Create non-default namespace.
	_, err := client.Namespaces().Register(&api.Namespace{Name: "prod"}, nil)
	must.NoError(t, err)

	// Register job with same name in both namespaces.
	evalIDs := []string{}

	jobDefault := testJob("test_job_restart")
	resp, _, err := client.Jobs().Register(jobDefault, nil)
	must.NoError(t, err)
	evalIDs = append(evalIDs, resp.EvalID)

	jobProd := testJob("test_job_restart")
	jobProd.Namespace = pointer.Of("prod")
	resp, _, err = client.Jobs().Register(jobProd, nil)
	must.NoError(t, err)
	evalIDs = append(evalIDs, resp.EvalID)

	jobUniqueProd := testJob("test_job_restart_prod_ns")
	jobUniqueProd.Namespace = pointer.Of("prod")
	resp, _, err = client.Jobs().Register(jobUniqueProd, nil)
	must.NoError(t, err)
	evalIDs = append(evalIDs, resp.EvalID)

	// Wait for evals to be processed.
	for _, evalID := range evalIDs {
		code := waitForSuccess(ui, client, fullId, t, evalID)
		must.Eq(t, 0, code)
	}
	ui.OutputWriter.Reset()

	testCases := []struct {
		name        string
		args        []string
		expectedErr string
	}{
		{
			name: "prefix match in default namespace",
			args: []string{"test_job"},
		},
		{
			name:        "invalid job",
			args:        []string{"not-valid"},
			expectedErr: "No job(s) with prefix or ID",
		},
		{
			name:        "prefix matches multiple jobs",
			args:        []string{"-namespace", "prod", "test_job"},
			expectedErr: "matched multiple jobs",
		},
		{
			name:        "prefix matches multiple jobs across namespaces",
			args:        []string{"-namespace", "*", "test_job"},
			expectedErr: "matched multiple jobs",
		},
		{
			name: "unique prefix match across namespaces",
			args: []string{"-namespace", "*", "test_job_restart_prod"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				ui.OutputWriter.Reset()
				ui.ErrorWriter.Reset()
			}()

			cmd := &JobRestartCommand{
				Meta: Meta{Ui: &cli.ConcurrentUi{Ui: ui}},
			}
			args := append([]string{"-address", url, "-yes"}, tc.args...)
			code := cmd.Run(args)

			if tc.expectedErr != "" {
				must.NonZero(t, code)
				must.StrContains(t, ui.ErrorWriter.String(), tc.expectedErr)
			} else {
				must.Zero(t, code)
			}
		})
	}
}

func TestJobRestartCommand_noAllocs(t *testing.T) {
	ci.Parallel(t)

	ui := cli.NewMockUi()
	cmd := &JobRestartCommand{Meta: Meta{Ui: ui}}

	// Start client and server and wait for node to be ready.
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	waitForNodes(t, client)

	// Register test job with impossible constraint so it doesn't get allocs.
	jobID := "test_job_restart_no_allocs"
	job := testJob(jobID)
	job.Datacenters = []string{"invalid"}

	resp, _, err := client.Jobs().Register(job, nil)
	must.NoError(t, err)

	code := waitForSuccess(ui, client, fullId, t, resp.EvalID)
	must.Eq(t, 2, code) // Placement is expected to fail so exit code is not 0.
	ui.OutputWriter.Reset()

	// Run job restart command and expect it to exit without restarts.
	code = cmd.Run([]string{
		"-address", url,
		"-yes",
		jobID,
	})
	must.Zero(t, code)
	must.StrContains(t, ui.OutputWriter.String(), "No allocations to restart")
}

func TestJobRestartCommand_rescheduleFail(t *testing.T) {
	ci.Parallel(t)

	ui := cli.NewMockUi()
	cmd := &JobRestartCommand{Meta: Meta{Ui: ui}}

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

	// Run job restart command and expect it to fail.
	code = cmd.Run([]string{
		"-address", url,
		"-batch-size", "2",
		"-reschedule",
		"-yes",
		jobID,
	})
	must.One(t, code)
	must.StrContains(t, ui.ErrorWriter.String(), "No nodes were eligible for evaluation")
}

func TestJobRestartCommand_monitorReplacementAlloc(t *testing.T) {
	ci.Parallel(t)

	ui := cli.NewMockUi()
	cmd := &JobRestartCommand{Meta: Meta{Ui: ui}}

	srv, client, _ := testServer(t, true, nil)
	defer srv.Shutdown()
	waitForNodes(t, client)

	// Register test job and update it twice so we end up with three
	// allocations, one replacing the next one.
	jobID := "test_job_restart_monitor_replacement"
	job := testJob(jobID)

	for i := 1; i <= 3; i++ {
		job.TaskGroups[0].Tasks[0].Config["run_for"] = fmt.Sprintf("%ds", i)
		resp, _, err := client.Jobs().Register(job, nil)
		must.NoError(t, err)

		code := waitForSuccess(ui, client, fullId, t, resp.EvalID)
		must.Zero(t, code)
	}
	ui.OutputWriter.Reset()

	// Prepare the command internals. We want to run a specific function and
	// target a specific allocation, so we can't run the full command.
	cmd.client = client
	cmd.verbose = true
	cmd.length = fullId

	// Fetch, sort, and monitor the oldest allocation.
	allocs, _, err := client.Jobs().Allocations(jobID, true, nil)
	must.NoError(t, err)
	sort.Slice(allocs, func(i, j int) bool {
		return allocs[i].CreateIndex < allocs[j].CreateIndex
	})

	errCh := make(chan error)
	go cmd.monitorReplacementAlloc(context.Background(), AllocationListStubWithJob{
		AllocationListStub: allocs[0],
		Job:                job,
	}, errCh)

	// Make sure the command doesn't get stuck and that we traverse the
	// follow-up allocations properly.
	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(func() error {
			select {
			case err := <-errCh:
				return err
			default:
				return fmt.Errorf("waiting for response")
			}
		}),
		wait.Timeout(time.Duration(testutil.TestMultiplier()*3)*time.Second),
	))
	must.StrContains(t, ui.OutputWriter.String(), fmt.Sprintf("%q replaced by %q", allocs[0].ID, allocs[1].ID))
	must.StrContains(t, ui.OutputWriter.String(), fmt.Sprintf("%q replaced by %q", allocs[1].ID, allocs[2].ID))
	must.StrContains(t, ui.OutputWriter.String(), fmt.Sprintf("%q is %q", allocs[2].ID, api.AllocClientStatusRunning))
}

func TestJobRestartCommand_activeDeployment(t *testing.T) {
	ci.Parallel(t)

	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()
	waitForNodes(t, client)

	// Register test job and update it once to trigger a deployment.
	jobID := "test_job_restart_deployment"
	job := testJob(jobID)
	job.Type = pointer.Of(api.JobTypeService)
	job.Update = &api.UpdateStrategy{
		Canary:      pointer.Of(1),
		AutoPromote: pointer.Of(false),
	}

	_, _, err := client.Jobs().Register(job, nil)
	must.NoError(t, err)

	_, _, err = client.Jobs().Register(job, nil)
	must.NoError(t, err)

	// Wait for a deployment to be running.
	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(func() error {
			deployments, _, err := client.Jobs().Deployments(jobID, true, nil)
			if err != nil {
				return err
			}
			for _, d := range deployments {
				if d.Status == api.DeploymentStatusRunning {
					return nil
				}
			}
			return fmt.Errorf("no running deployments")
		}),
		wait.Timeout(time.Duration(testutil.TestMultiplier()*3)*time.Second),
	))

	// Run job restart command and expect it to fail.
	ui := cli.NewMockUi()
	cmd := &JobRestartCommand{Meta: Meta{Ui: ui}}

	code := cmd.Run([]string{
		"-address", url,
		"-on-error", jobRestartOnErrorFail,
		"-verbose",
		jobID,
	})
	must.One(t, code)
	must.RegexMatch(t, regexp.MustCompile(`Deployment .+ is "running"`), ui.ErrorWriter.String())
}

func TestJobRestartCommand_ACL(t *testing.T) {
	ci.Parallel(t)

	// Start server with ACL enabled.
	srv, client, url := testServer(t, true, func(c *agent.Config) {
		c.ACL.Enabled = true
	})
	defer srv.Shutdown()

	rootTokenOpts := &api.WriteOptions{
		AuthToken: srv.RootToken.SecretID,
	}

	// Register test job.
	jobID := "test_job_restart_acl"
	job := testJob(jobID)
	_, _, err := client.Jobs().Register(job, rootTokenOpts)
	must.NoError(t, err)

	// Wait for allocs to be running.
	waitForJobAllocsStatus(t, client, jobID, api.AllocClientStatusRunning, srv.RootToken.SecretID)

	testCases := []struct {
		name        string
		jobPrefix   bool
		aclPolicy   string
		expectedErr string
	}{
		{
			name:        "no token",
			aclPolicy:   "",
			expectedErr: api.PermissionDeniedErrorContent,
		},
		{
			name: "alloc-lifecycle not enough",
			aclPolicy: `
namespace "default" {
	capabilities = ["alloc-lifecycle"]
}
`,
			expectedErr: api.PermissionDeniedErrorContent,
		},
		{
			name: "read-job not enough",
			aclPolicy: `
namespace "default" {
	capabilities = ["read-job"]
}
`,
			expectedErr: api.PermissionDeniedErrorContent,
		},
		{
			name: "alloc-lifecycle and read-job allowed",
			aclPolicy: `
namespace "default" {
	capabilities = ["alloc-lifecycle", "read-job"]
}
`,
		},
		{
			name: "job prefix requires list-jobs",
			aclPolicy: `
namespace "default" {
	capabilities = ["alloc-lifecycle", "read-job"]
}
`,
			jobPrefix:   true,
			expectedErr: "job not found",
		},
		{
			name: "job prefix works with list-jobs",
			aclPolicy: `
namespace "default" {
	capabilities = ["list-jobs", "alloc-lifecycle", "read-job"]
}
`,
			jobPrefix: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ui := cli.NewMockUi()
			cmd := &JobRestartCommand{Meta: Meta{Ui: ui}}
			args := []string{
				"-address", url,
				"-yes",
			}

			if tc.aclPolicy != "" {
				// Create ACL token with test case policy.
				policy := &api.ACLPolicy{
					Name:  nonAlphaNum.ReplaceAllString(tc.name, "-"),
					Rules: tc.aclPolicy,
				}
				_, err := client.ACLPolicies().Upsert(policy, rootTokenOpts)
				must.NoError(t, err)

				token := &api.ACLToken{
					Type:     "client",
					Policies: []string{policy.Name},
				}
				token, _, err = client.ACLTokens().Create(token, rootTokenOpts)
				must.NoError(t, err)

				// Set token in command args.
				args = append(args, "-token", token.SecretID)
			}

			// Add job ID or job ID prefix to the command.
			if tc.jobPrefix {
				args = append(args, jobID[0:3])
			} else {
				args = append(args, jobID)
			}

			// Run command.
			code := cmd.Run(args)
			if tc.expectedErr == "" {
				must.Zero(t, code)
			} else {
				must.One(t, code)
				must.StrContains(t, ui.ErrorWriter.String(), tc.expectedErr)
			}
		})
	}
}

// TODO(luiz): update once alloc restart supports -no-shutdown-delay.
func TestJobRestartCommand_shutdownDelay_reschedule(t *testing.T) {
	ci.Parallel(t)

	// Start client and server and wait for node to be ready.
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	waitForNodes(t, client)

	testCases := []struct {
		name          string
		args          []string
		shutdownDelay bool
	}{
		{
			name:          "job reschedule with shutdown delay by default",
			args:          []string{"-reschedule"},
			shutdownDelay: true,
		},
		{
			name:          "job reschedule no shutdown delay",
			args:          []string{"-reschedule", "-no-shutdown-delay"},
			shutdownDelay: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ui := cli.NewMockUi()
			cmd := &JobRestartCommand{Meta: Meta{Ui: ui}}

			// Register job with 2 allocations and shutdown_delay.
			shutdownDelay := 3 * time.Second
			jobID := nonAlphaNum.ReplaceAllString(tc.name, "-")

			job := testJob(jobID)
			job.TaskGroups[0].Count = pointer.Of(2)
			job.TaskGroups[0].Tasks[0].Config["run_for"] = "10m"
			job.TaskGroups[0].Tasks[0].ShutdownDelay = shutdownDelay
			job.TaskGroups[0].Tasks[0].Services = []*api.Service{{
				Name:     "service",
				Provider: "nomad",
			}}

			resp, _, err := client.Jobs().Register(job, nil)
			must.NoError(t, err)

			code := waitForSuccess(ui, client, fullId, t, resp.EvalID)
			must.Zero(t, code)
			ui.OutputWriter.Reset()

			// Wait for alloc to be running.
			allocStubs, _, err := client.Jobs().Allocations(jobID, true, nil)
			must.NoError(t, err)
			for _, alloc := range allocStubs {
				waitForAllocRunning(t, client, alloc.ID)
			}

			// Add address and job ID to the command and run.
			args := []string{
				"-address", url,
				"-batch-size", "1",
				"-batch-wait", "0",
				"-yes",
			}
			args = append(args, tc.args...)
			args = append(args, jobID)

			code = cmd.Run(args)
			must.Zero(t, code)

			// Wait for all allocs to restart.
			reschedules := map[string]bool{}
			for _, alloc := range allocStubs {
				reschedules[alloc.ID] = true
			}
			allocs := waitAllocsRescheduled(t, client, reschedules)

			// Check that allocs have shutdown delay event.
			for _, alloc := range allocs {
				for _, s := range alloc.TaskStates {
					var killedEv *api.TaskEvent
					var killingEv *api.TaskEvent
					for _, ev := range s.Events {
						if strings.Contains(ev.Type, "Killed") {
							killedEv = ev
						}
						if strings.Contains(ev.Type, "Killing") {
							killingEv = ev
						}
					}

					diff := killedEv.Time - killingEv.Time
					if tc.shutdownDelay {
						must.GreaterEq(t, shutdownDelay, time.Duration(diff))
					} else {
						// Add a bit of slack to account for the actual
						// shutdown time of the task.
						must.Between(t, shutdownDelay, time.Duration(diff), shutdownDelay+time.Second)
					}
				}
			}
		})
	}
}

func TestJobRestartCommand_filterAllocs(t *testing.T) {
	ci.Parallel(t)

	task1 := api.NewTask("task_1", "mock_driver")
	task2 := api.NewTask("task_2", "mock_driver")
	task3 := api.NewTask("task_3", "mock_driver")

	jobV1 := api.NewServiceJob("example", "example", "global", 1).
		AddTaskGroup(
			api.NewTaskGroup("group_1", 1).
				AddTask(task1),
		).
		AddTaskGroup(
			api.NewTaskGroup("group_2", 1).
				AddTask(task1).
				AddTask(task2),
		).
		AddTaskGroup(
			api.NewTaskGroup("group_3", 1).
				AddTask(task3),
		)
	jobV1.Version = pointer.Of(uint64(1))

	jobV2 := api.NewServiceJob("example", "example", "global", 1).
		AddTaskGroup(
			api.NewTaskGroup("group_1", 1).
				AddTask(task1),
		).
		AddTaskGroup(
			api.NewTaskGroup("group_2", 1).
				AddTask(task2),
		)
	jobV2.Version = pointer.Of(uint64(2))

	allAllocs := []AllocationListStubWithJob{}
	allocs := map[string]AllocationListStubWithJob{}
	for _, job := range []*api.Job{jobV1, jobV2} {
		for _, tg := range job.TaskGroups {
			for _, desired := range []string{api.AllocDesiredStatusRun, api.AllocDesiredStatusStop} {
				for _, client := range []string{api.AllocClientStatusRunning, api.AllocClientStatusComplete} {
					key := fmt.Sprintf("job_v%d_%s_%s_%s", *job.Version, *tg.Name, desired, client)
					alloc := AllocationListStubWithJob{
						AllocationListStub: &api.AllocationListStub{
							ID:            key,
							JobVersion:    *job.Version,
							TaskGroup:     *tg.Name,
							DesiredStatus: desired,
							ClientStatus:  client,
						},
						Job: job,
					}
					allocs[key] = alloc
					allAllocs = append(allAllocs, alloc)
				}
			}
		}
	}

	testCases := []struct {
		name           string
		args           []string
		expectedAllocs []AllocationListStubWithJob
	}{
		{
			name: "skip by group",
			args: []string{"-group", "group_1"},
			expectedAllocs: []AllocationListStubWithJob{
				allocs["job_v1_group_1_run_running"],
				allocs["job_v1_group_1_run_complete"],
				allocs["job_v1_group_1_stop_running"],
				allocs["job_v2_group_1_run_running"],
				allocs["job_v2_group_1_run_complete"],
				allocs["job_v2_group_1_stop_running"],
			},
		},
		{
			name: "skip by old group",
			args: []string{"-group", "group_3"},
			expectedAllocs: []AllocationListStubWithJob{
				allocs["job_v1_group_3_run_running"],
				allocs["job_v1_group_3_run_complete"],
				allocs["job_v1_group_3_stop_running"],
			},
		},
		{
			name: "skip by task",
			args: []string{"-task", "task_2"},
			expectedAllocs: []AllocationListStubWithJob{
				allocs["job_v1_group_2_run_running"],
				allocs["job_v1_group_2_run_complete"],
				allocs["job_v1_group_2_stop_running"],
				allocs["job_v2_group_2_run_running"],
				allocs["job_v2_group_2_run_complete"],
				allocs["job_v2_group_2_stop_running"],
			},
		},
		{
			name: "skip by old task",
			args: []string{"-task", "task_3"},
			expectedAllocs: []AllocationListStubWithJob{
				allocs["job_v1_group_3_run_running"],
				allocs["job_v1_group_3_run_complete"],
				allocs["job_v1_group_3_stop_running"],
			},
		},
		{
			name: "skip by group and task",
			args: []string{
				"-group", "group_1",
				"-group", "group_2",
				"-task", "task_2",
			},
			// Only group_2 has task_2 in all job versions.
			expectedAllocs: []AllocationListStubWithJob{
				allocs["job_v1_group_2_run_running"],
				allocs["job_v1_group_2_run_complete"],
				allocs["job_v1_group_2_stop_running"],
				allocs["job_v2_group_2_run_running"],
				allocs["job_v2_group_2_run_complete"],
				allocs["job_v2_group_2_stop_running"],
			},
		},
		{
			name: "skip by status",
			args: []string{},
			expectedAllocs: []AllocationListStubWithJob{
				allocs["job_v1_group_1_run_running"],
				allocs["job_v1_group_1_run_complete"],
				allocs["job_v1_group_1_stop_running"],
				allocs["job_v1_group_2_run_running"],
				allocs["job_v1_group_2_run_complete"],
				allocs["job_v1_group_2_stop_running"],
				allocs["job_v1_group_3_run_running"],
				allocs["job_v1_group_3_run_complete"],
				allocs["job_v1_group_3_stop_running"],
				allocs["job_v2_group_1_run_running"],
				allocs["job_v2_group_1_run_complete"],
				allocs["job_v2_group_1_stop_running"],
				allocs["job_v2_group_2_run_running"],
				allocs["job_v2_group_2_run_complete"],
				allocs["job_v2_group_2_stop_running"],
			},
		},
		{
			name:           "no matches by group",
			args:           []string{"-group", "group_404"},
			expectedAllocs: []AllocationListStubWithJob{},
		},
		{
			name:           "no matches by task",
			args:           []string{"-task", "task_404"},
			expectedAllocs: []AllocationListStubWithJob{},
		},
		{
			name: "no matches by task with group",
			args: []string{
				"-group", "group_1",
				"-task", "task_2", // group_1 never has task_2.
			},
			expectedAllocs: []AllocationListStubWithJob{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ui := cli.NewMockUi()
			cmd := &JobRestartCommand{
				Meta: Meta{Ui: &cli.ConcurrentUi{Ui: ui}},
			}

			args := append(tc.args, "-verbose", "-yes", "example")
			code, err := cmd.parseAndValidate(args)
			must.NoError(t, err)
			must.Zero(t, code)

			got := cmd.filterAllocs(allAllocs)
			must.SliceEqFunc(t, tc.expectedAllocs, got, func(a, b AllocationListStubWithJob) bool {
				return a.ID == b.ID
			})

			expected := set.FromFunc(tc.expectedAllocs, func(a AllocationListStubWithJob) string {
				return a.ID
			})
			for _, a := range allAllocs {
				if !expected.Contains(a.ID) {
					must.StrContains(t, ui.OutputWriter.String(), fmt.Sprintf("Skipping allocation %q", a.ID))
				}
			}
		})
	}
}

func TestJobRestartCommand_onErrorFail(t *testing.T) {
	ci.Parallel(t)

	ui := cli.NewMockUi()
	cmd := &JobRestartCommand{Meta: Meta{Ui: ui}}

	// Start client and server and wait for node to be ready.
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	parsedURL, err := neturl.Parse(url)
	must.NoError(t, err)

	waitForNodes(t, client)

	// Register a job with 3 allocations.
	jobID := "test_job_restart_command_fail_on_error"
	job := testJob(jobID)
	job.TaskGroups[0].Count = pointer.Of(3)

	resp, _, err := client.Jobs().Register(job, nil)
	must.NoError(t, err)

	code := waitForSuccess(ui, client, fullId, t, resp.EvalID)
	must.Zero(t, code)
	ui.OutputWriter.Reset()

	// Create a proxy to inject an error after 2 allocation restarts.
	// Also counts how many restart requests are made so we can check that the
	// command stops after the error happens.
	var allocRestarts int32
	proxy := httptest.NewServer(&httputil.ReverseProxy{
		ModifyResponse: func(resp *http.Response) error {
			if strings.HasSuffix(resp.Request.URL.Path, "/restart") {
				count := atomic.AddInt32(&allocRestarts, 1)
				if count == 2 {
					return fmt.Errorf("fail")
				}
			}
			return nil
		},
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(parsedURL)
		},
	})
	defer proxy.Close()

	// Run command with -fail-on-error.
	// Expect only 2 restarts requests even though there are 3 allocations.
	code = cmd.Run([]string{
		"-address", proxy.URL,
		"-on-error", jobRestartOnErrorFail,
		jobID,
	})
	must.One(t, code)
	must.Eq(t, 2, allocRestarts)
}

// waitTasksRestarted blocks until the given allocations have restarted or not.
// Returns a list with updated state of the allocations.
//
// To determine if a restart happened the function looks for a "Restart
// Signaled" event in the list of task events. Allocations that are reused
// between tests may contain a restart event from a past test case, leading to
// false positives.
//
// The restarts map contains values structured as group:task:<expect restart?>.
func waitTasksRestarted(
	t *testing.T,
	client *api.Client,
	allocs []*api.AllocationListStub,
	restarts map[string]map[string]bool,
) []*api.Allocation {
	t.Helper()

	var newAllocs []*api.Allocation
	testutil.WaitForResult(func() (bool, error) {
		newAllocs = make([]*api.Allocation, 0, len(allocs))

		for _, alloc := range allocs {
			if _, ok := restarts[alloc.TaskGroup]; !ok {
				t.Fatalf("Missing group %q in restarts map", alloc.TaskGroup)
			}

			// Skip allocations that are not supposed to be running.
			if alloc.DesiredStatus != api.AllocDesiredStatusRun {
				continue
			}

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

				if restarted && !restarts[updated.TaskGroup][task] {
					return false, fmt.Errorf(
						"task %q in alloc %s for group %q not expected to restart",
						task, updated.ID, updated.TaskGroup,
					)
				}
				if !restarted && restarts[updated.TaskGroup][task] {
					return false, fmt.Errorf(
						"task %q in alloc %s for group %q expected to restart but didn't",
						task, updated.ID, updated.TaskGroup,
					)
				}
			}
		}
		return true, nil
	}, func(err error) {
		must.NoError(t, err)
	})

	return newAllocs
}

// waitAllocsRescheduled blocks until the given allocations have been
// rescueduled or not. Returns a list with updated state of the allocations.
//
// To determined if an allocation has been rescheduled the function looks for
// a non-empty NextAllocation field.
//
// The reschedules map maps allocation IDs to a boolean indicating if a
// reschedule is expected for that allocation.
func waitAllocsRescheduled(t *testing.T, client *api.Client, reschedules map[string]bool) []*api.Allocation {
	t.Helper()

	var newAllocs []*api.Allocation
	testutil.WaitForResult(func() (bool, error) {
		newAllocs = make([]*api.Allocation, 0, len(reschedules))

		for allocID, reschedule := range reschedules {
			alloc, _, err := client.Allocations().Info(allocID, nil)
			if err != nil {
				return false, err
			}
			newAllocs = append(newAllocs, alloc)

			wasRescheduled := alloc.NextAllocation != ""
			if wasRescheduled && !reschedule {
				return false, fmt.Errorf("alloc %s not expected to be rescheduled", alloc.ID)
			}
			if !wasRescheduled && reschedule {
				return false, fmt.Errorf("alloc %s expected to be rescheduled but wasn't", alloc.ID)
			}
		}
		return true, nil
	}, func(err error) {
		must.NoError(t, err)
	})

	return newAllocs
}

// getRestartBatches returns a list of allocations per batch of restarts.
//
// Since restarts are issued concurrently, it's expected that allocations in
// the same batch have fairly close LastRestart times, so a 1s delay between
// restarts may be enough to indicate a new batch.
func getRestartBatches(allocs []*api.Allocation, groups []string, task string) [][]*api.Allocation {
	groupsSet := set.From(groups)
	batches := [][]*api.Allocation{}

	type allocRestart struct {
		alloc   *api.Allocation
		restart time.Time
	}

	restarts := make([]allocRestart, 0, len(allocs))
	for _, alloc := range allocs {
		if !groupsSet.Contains(alloc.TaskGroup) {
			continue
		}

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

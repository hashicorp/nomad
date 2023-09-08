// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobStatusCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &JobStatusCommand{}
}

func TestJobStatusCommand_Run(t *testing.T) {
	ci.Parallel(t)
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()
	testutil.WaitForResult(func() (bool, error) {
		nodes, _, err := client.Nodes().List(nil)
		if err != nil {
			return false, err
		}
		if len(nodes) == 0 {
			return false, fmt.Errorf("missing node")
		}
		if _, ok := nodes[0].Drivers["mock_driver"]; !ok {
			return false, fmt.Errorf("mock_driver not ready")
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %s", err)
	})

	ui := cli.NewMockUi()
	cmd := &JobStatusCommand{Meta: Meta{Ui: ui}}

	// Should return blank for no jobs
	if code := cmd.Run([]string{"-address=" + url}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}

	// Check for this awkward nil string, since a nil bytes.Buffer
	// returns this purposely, and mitchellh/cli has a nil pointer
	// if nothing was ever output.
	exp := "No running jobs"
	if out := strings.TrimSpace(ui.OutputWriter.String()); out != exp {
		t.Fatalf("expected %q; got: %q", exp, out)
	}

	// Register two jobs
	job1 := testJob("job1_sfx")
	resp, _, err := client.Jobs().Register(job1, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if code := waitForSuccess(ui, client, fullId, t, resp.EvalID); code != 0 {
		t.Fatalf("status code non zero saw %d", code)
	}

	job2 := testJob("job2_sfx")
	resp2, _, err := client.Jobs().Register(job2, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if code := waitForSuccess(ui, client, fullId, t, resp2.EvalID); code != 0 {
		t.Fatalf("status code non zero saw %d", code)
	}

	// Query again and check the result
	if code := cmd.Run([]string{"-address=" + url}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	out := ui.OutputWriter.String()
	if !strings.Contains(out, "job1_sfx") || !strings.Contains(out, "job2_sfx") {
		t.Fatalf("expected job1_sfx and job2_sfx, got: %s", out)
	}
	ui.OutputWriter.Reset()

	// Query a single job
	if code := cmd.Run([]string{"-address=" + url, "job2_sfx"}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	out = ui.OutputWriter.String()
	if strings.Contains(out, "job1_sfx") || !strings.Contains(out, "job2_sfx") {
		t.Fatalf("expected only job2_sfx, got: %s", out)
	}
	if !strings.Contains(out, "Allocations") {
		t.Fatalf("should dump allocations")
	}
	if !strings.Contains(out, "Summary") {
		t.Fatalf("should dump summary")
	}
	ui.OutputWriter.Reset()

	// Query a single job showing evals
	if code := cmd.Run([]string{"-address=" + url, "-evals", "job2_sfx"}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	out = ui.OutputWriter.String()
	if strings.Contains(out, "job1_sfx") || !strings.Contains(out, "job2_sfx") {
		t.Fatalf("expected only job2_sfx, got: %s", out)
	}
	if !strings.Contains(out, "Evaluations") {
		t.Fatalf("should dump evaluations")
	}
	if !strings.Contains(out, "Allocations") {
		t.Fatalf("should dump allocations")
	}
	ui.OutputWriter.Reset()

	// Query a single job in verbose mode
	if code := cmd.Run([]string{"-address=" + url, "-verbose", "job2_sfx"}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}

	nodeName := ""
	if allocs, _, err := client.Jobs().Allocations("job2_sfx", false, nil); err == nil {
		if len(allocs) > 0 {
			nodeName = allocs[0].NodeName
		} else {
			t.Fatalf("no running allocations")
		}
	}

	out = ui.OutputWriter.String()
	if strings.Contains(out, "job1_sfx") || !strings.Contains(out, "job2_sfx") {
		t.Fatalf("expected only job2_sfx, got: %s", out)
	}
	if !strings.Contains(out, "Evaluations") {
		t.Fatalf("should dump evaluations")
	}
	if !strings.Contains(out, "Allocations") {
		t.Fatalf("should dump allocations")
	}
	if !strings.Contains(out, "Created") {
		t.Fatal("should have created header")
	}
	if !strings.Contains(out, "Modified") {
		t.Fatal("should have modified header")
	}

	// string calculations based on 1-byte chars, not using runes
	allocationsTable := strings.Split(out, "Allocations\n")
	if len(allocationsTable) == 1 {
		t.Fatal("no running allocations")
	}
	allocationsTableStr := allocationsTable[1]

	nodeNameHeaderStr := "Node Name"
	nodeNameHeaderIndex := strings.Index(allocationsTableStr, nodeNameHeaderStr)
	nodeNameRegexpStr := fmt.Sprintf(`.*%s.*\n.{%d}%s`, nodeNameHeaderStr, nodeNameHeaderIndex, regexp.QuoteMeta(nodeName))
	require.Regexp(t, regexp.MustCompile(nodeNameRegexpStr), out)

	ui.ErrorWriter.Reset()
	ui.OutputWriter.Reset()

	// Query jobs with prefix match
	if code := cmd.Run([]string{"-address=" + url, "job"}); code != 1 {
		t.Fatalf("expected exit 1, got: %d", code)
	}
	out = ui.ErrorWriter.String()
	if !strings.Contains(out, "job1_sfx") || !strings.Contains(out, "job2_sfx") {
		t.Fatalf("expected job1_sfx and job2_sfx, got: %s", out)
	}
	ui.ErrorWriter.Reset()
	ui.OutputWriter.Reset()

	// Query a single job with prefix match
	if code := cmd.Run([]string{"-address=" + url, "job1"}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	out = ui.OutputWriter.String()
	if !strings.Contains(out, "job1_sfx") || strings.Contains(out, "job2_sfx") {
		t.Fatalf("expected only job1_sfx, got: %s", out)
	}

	if !strings.Contains(out, "Created") {
		t.Fatal("should have created header")
	}

	if !strings.Contains(out, "Modified") {
		t.Fatal("should have modified header")
	}
	ui.OutputWriter.Reset()

	// Query in short view mode
	if code := cmd.Run([]string{"-address=" + url, "-short", "job2"}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	out = ui.OutputWriter.String()
	if !strings.Contains(out, "job2") {
		t.Fatalf("expected job2, got: %s", out)
	}
	if strings.Contains(out, "Evaluations") {
		t.Fatalf("should not dump evaluations")
	}
	if strings.Contains(out, "Allocations") {
		t.Fatalf("should not dump allocations")
	}
	if strings.Contains(out, resp.EvalID) {
		t.Fatalf("should not contain full identifiers, got %s", out)
	}
	ui.OutputWriter.Reset()

	// Request full identifiers
	if code := cmd.Run([]string{"-address=" + url, "-verbose", "job1"}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	out = ui.OutputWriter.String()
	if !strings.Contains(out, resp.EvalID) {
		t.Fatalf("should contain full identifiers, got %s", out)
	}
}

func TestJobStatusCommand_Fails(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &JobStatusCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	if code := cmd.Run([]string{"some", "bad", "args"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, commandErrorText(cmd)) {
		t.Fatalf("expected help output, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fails on connection failure
	if code := cmd.Run([]string{"-address=nope"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Error querying jobs") {
		t.Fatalf("expected failed query error, got: %s", out)
	}
}

func TestJobStatusCommand_AutocompleteArgs(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)

	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &JobStatusCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Create a fake job
	state := srv.Agent.Server().State()
	j := mock.Job()
	assert.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, j))

	prefix := j.ID[:len(j.ID)-5]
	args := complete.Args{Last: prefix}
	predictor := cmd.AutocompleteArgs()

	res := predictor.Predict(args)
	assert.Equal(1, len(res))
	assert.Equal(j.ID, res[0])
}

func TestJobStatusCommand_WithAccessPolicy(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)

	config := func(c *agent.Config) {
		c.ACL.Enabled = true
	}

	srv, client, url := testServer(t, true, config)
	defer srv.Shutdown()

	// Bootstrap an initial ACL token
	token := srv.RootToken
	assert.NotNil(token, "failed to bootstrap ACL token")

	// Wait for client ready
	client.SetSecretID(token.SecretID)
	testutil.WaitForResult(func() (bool, error) {
		nodes, _, err := client.Nodes().List(nil)
		if err != nil {
			return false, err
		}
		if len(nodes) == 0 {
			return false, fmt.Errorf("missing node")
		}
		if _, ok := nodes[0].Drivers["mock_driver"]; !ok {
			return false, fmt.Errorf("mock_driver not ready")
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %s", err)
	})

	// Register a job
	j := testJob("job1_sfx")

	invalidToken := mock.ACLToken()

	ui := cli.NewMockUi()
	cmd := &JobStatusCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// registering a job without a token fails
	client.SetSecretID(invalidToken.SecretID)
	resp, _, err := client.Jobs().Register(j, nil)
	assert.NotNil(err)

	// registering a job with a valid token succeeds
	client.SetSecretID(token.SecretID)
	resp, _, err = client.Jobs().Register(j, nil)
	assert.Nil(err)
	code := waitForSuccess(ui, client, fullId, t, resp.EvalID)
	assert.Equal(0, code)

	// Request Job List without providing a valid token
	code = cmd.Run([]string{"-address=" + url, "-token=" + invalidToken.SecretID, "-short"})
	assert.Equal(1, code)

	// Request Job List with a valid token
	code = cmd.Run([]string{"-address=" + url, "-token=" + token.SecretID, "-short"})
	assert.Equal(0, code)

	out := ui.OutputWriter.String()
	if !strings.Contains(out, *j.ID) {
		t.Fatalf("should contain full identifiers, got %s", out)
	}
}

func TestJobStatusCommand_RescheduleEvals(t *testing.T) {
	ci.Parallel(t)
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	// Wait for a node to be ready
	testutil.WaitForResult(func() (bool, error) {
		nodes, _, err := client.Nodes().List(nil)
		if err != nil {
			return false, err
		}
		for _, node := range nodes {
			if node.Status == structs.NodeStatusReady {
				return true, nil
			}
		}
		return false, fmt.Errorf("no ready nodes")
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	ui := cli.NewMockUi()
	cmd := &JobStatusCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	require := require.New(t)
	state := srv.Agent.Server().State()

	// Create state store objects for job, alloc and followup eval with a future WaitUntil value
	j := mock.Job()
	require.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 900, nil, j))

	e := mock.Eval()
	e.WaitUntil = time.Now().Add(1 * time.Hour)
	require.Nil(state.UpsertEvals(structs.MsgTypeTestSetup, 902, []*structs.Evaluation{e}))
	a := mock.Alloc()
	a.Job = j
	a.JobID = j.ID
	a.TaskGroup = j.TaskGroups[0].Name
	a.FollowupEvalID = e.ID
	a.Metrics = &structs.AllocMetric{}
	a.DesiredStatus = structs.AllocDesiredStatusRun
	a.ClientStatus = structs.AllocClientStatusRunning
	require.Nil(state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{a}))

	// Query jobs with prefix match
	if code := cmd.Run([]string{"-address=" + url, j.ID}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	out := ui.OutputWriter.String()
	require.Contains(out, "Future Rescheduling Attempts")
	require.Contains(out, e.ID[:8])
}

func TestJobStatusCommand_ACL(t *testing.T) {
	ci.Parallel(t)

	// Start server with ACL enabled.
	srv, _, url := testServer(t, true, func(c *agent.Config) {
		c.ACL.Enabled = true
	})
	defer srv.Shutdown()

	// Create a job.
	job := mock.MinJob()
	state := srv.Agent.Server().State()
	err := state.UpsertJob(structs.MsgTypeTestSetup, 100, nil, job)
	must.NoError(t, err)

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
			name: "missing read-job",
			aclPolicy: `
namespace "default" {
	capabilities = ["submit-job"]
}
`,
			expectedErr: api.PermissionDeniedErrorContent,
		},
		{
			name: "read-job allowed",
			aclPolicy: `
namespace "default" {
	capabilities = ["read-job"]
}
`,
		},
		{
			name:      "job prefix requires list-job",
			jobPrefix: true,
			aclPolicy: `
namespace "default" {
	capabilities = ["read-job"]
}
`,
			expectedErr: "job not found",
		},
		{
			name:      "job prefix works with list-job",
			jobPrefix: true,
			aclPolicy: `
namespace "default" {
	capabilities = ["read-job", "list-jobs"]
}
`,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ui := cli.NewMockUi()
			cmd := &JobStatusCommand{Meta: Meta{Ui: ui}}
			args := []string{
				"-address", url,
			}

			if tc.aclPolicy != "" {
				// Create ACL token with test case policy and add it to the
				// command.
				policyName := nonAlphaNum.ReplaceAllString(tc.name, "-")
				token := mock.CreatePolicyAndToken(t, state, uint64(302+i), policyName, tc.aclPolicy)
				args = append(args, "-token", token.SecretID)
			}

			// Add job ID or job ID prefix to the command.
			if tc.jobPrefix {
				args = append(args, job.ID[:3])
			} else {
				args = append(args, job.ID)
			}
			code := cmd.Run(args)

			// Run command.
			if tc.expectedErr == "" {
				must.Zero(t, code)
			} else {
				must.One(t, code)
				must.StrContains(t, ui.ErrorWriter.String(), tc.expectedErr)
			}
		})
	}
}

func waitForSuccess(ui cli.Ui, client *api.Client, length int, t *testing.T, evalId string) int {
	mon := newMonitor(ui, client, length)
	monErr := mon.monitor(evalId)
	return monErr
}

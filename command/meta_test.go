// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"flag"
	"fmt"
	"os"
	"reflect"
	"sort"
	"testing"

	"github.com/creack/pty"
	"github.com/hashicorp/cli"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/shoenig/test/must"
)

func TestMeta_FlagSet(t *testing.T) {
	ci.Parallel(t)
	cases := []struct {
		Flags    FlagSetFlags
		Expected []string
	}{
		{
			FlagSetNone,
			[]string{},
		},
		{
			FlagSetClient,
			[]string{
				"address",
				"no-color",
				"force-color",
				"region",
				"namespace",
				"ca-cert",
				"ca-path",
				"client-cert",
				"client-key",
				"insecure",
				"tls-server-name",
				"tls-skip-verify",
				"token",
			},
		},
	}

	for i, tc := range cases {
		var m Meta
		fs := m.FlagSet("foo", tc.Flags)

		actual := make([]string, 0, 0)
		fs.VisitAll(func(f *flag.Flag) {
			actual = append(actual, f.Name)
		})
		sort.Strings(actual)
		sort.Strings(tc.Expected)

		if !reflect.DeepEqual(actual, tc.Expected) {
			t.Fatalf("%d: flags: %#v\n\nExpected: %#v\nGot: %#v",
				i, tc.Flags, tc.Expected, actual)
		}
	}
}

func TestMeta_Colorize(t *testing.T) {

	type testCaseSetupFn func(*testing.T, *Meta)

	cases := []struct {
		Name        string
		SetupFn     testCaseSetupFn
		ExpectColor bool
	}{
		{
			Name:        "disable colors if UI is not colored",
			ExpectColor: false,
		},
		{
			Name: "colors if UI is colored",
			SetupFn: func(t *testing.T, m *Meta) {
				m.Ui = &cli.ColoredUi{}
			},
			ExpectColor: true,
		},
		{
			Name: "disable colors via CLI flag",
			SetupFn: func(t *testing.T, m *Meta) {
				m.SetupUi([]string{"-no-color"})
			},
			ExpectColor: false,
		},
		{
			Name: "disable colors via env var",
			SetupFn: func(t *testing.T, m *Meta) {
				t.Setenv(EnvNomadCLINoColor, "1")
				m.SetupUi([]string{})
			},
			ExpectColor: false,
		},
		{
			Name: "force colors via CLI flag",
			SetupFn: func(t *testing.T, m *Meta) {
				m.SetupUi([]string{"-force-color"})
			},
			ExpectColor: true,
		},
		{
			Name: "force colors via env var",
			SetupFn: func(t *testing.T, m *Meta) {
				t.Setenv(EnvNomadCLIForceColor, "1")
				m.SetupUi([]string{})
			},
			ExpectColor: true,
		},
		{
			Name: "no color take predecence over force color via CLI flag",
			SetupFn: func(t *testing.T, m *Meta) {
				m.SetupUi([]string{"-no-color", "-force-color"})
			},
			ExpectColor: false,
		},
		{
			Name: "no color take predecence over force color via env var",
			SetupFn: func(t *testing.T, m *Meta) {
				t.Setenv(EnvNomadCLINoColor, "1")
				m.SetupUi([]string{"-force-color"})
			},
			ExpectColor: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			// Create fake test terminal.
			_, tty, err := pty.Open()
			must.NoError(t, err)
			defer tty.Close()

			oldStdout := os.Stdout
			defer func() { os.Stdout = oldStdout }()
			os.Stdout = tty

			// Make sure color related environment variables are clean.
			t.Setenv(EnvNomadCLIForceColor, "")
			t.Setenv(EnvNomadCLINoColor, "")

			// Run test case.
			m := &Meta{}
			if tc.SetupFn != nil {
				tc.SetupFn(t, m)
			}

			must.Eq(t, !tc.ExpectColor, m.Colorize().Disable)
		})
	}
}

func TestMeta_JobByPrefix(t *testing.T) {
	ci.Parallel(t)

	srv, client, _ := testServer(t, true, nil)
	defer srv.Shutdown()

	// Wait for a node to be ready
	waitForNodes(t, client)

	ui := cli.NewMockUi()
	meta := &Meta{Ui: ui, namespace: api.AllNamespacesNamespace}
	client.SetNamespace(api.AllNamespacesNamespace)

	jobs := []struct {
		namespace string
		id        string
	}{
		{namespace: "default", id: "example"},
		{namespace: "default", id: "job"},
		{namespace: "default", id: "job-1"},
		{namespace: "default", id: "job-2"},
		{namespace: "prod", id: "job-1"},
	}
	for _, j := range jobs {
		job := testJob(j.id)
		job.Namespace = pointer.Of(j.namespace)

		_, err := client.Namespaces().Register(&api.Namespace{Name: j.namespace}, nil)
		must.NoError(t, err)

		w := &api.WriteOptions{Namespace: j.namespace}
		resp, _, err := client.Jobs().Register(job, w)
		must.NoError(t, err)

		code := waitForSuccess(ui, client, fullId, t, resp.EvalID)
		must.Zero(t, code)
	}

	testCases := []struct {
		name          string
		prefix        string
		filterFunc    JobByPrefixFilterFunc
		expectedError string
	}{
		{
			name:   "exact match",
			prefix: "job",
		},
		{
			name:   "partial match",
			prefix: "exam",
		},
		{
			name:   "match with filter",
			prefix: "job-",
			filterFunc: func(j *api.JobListStub) bool {
				// Filter out jobs with "job-" so that only "job-2" matches.
				return j.ID == "job-2"
			},
		},
		{
			name:          "multiple matches",
			prefix:        "job-",
			expectedError: "matched multiple jobs",
		},
		{
			name:          "no match",
			prefix:        "not-found",
			expectedError: "No job(s) with prefix or ID",
		},
		{
			name:          "multiple matches across namespaces",
			prefix:        "job-1",
			expectedError: "matched multiple jobs",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			job, err := meta.JobByPrefix(client, tc.prefix, tc.filterFunc)
			if tc.expectedError != "" {
				must.Nil(t, job)
				must.ErrorContains(t, err, tc.expectedError)
			} else {
				must.NoError(t, err)
				must.NotNil(t, job)
				must.StrContains(t, *job.ID, tc.prefix)
			}
		})
	}
}

func TestMeta_ShowUIPath(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name           string
		context        UIHintContext
		expectedURL    string
		expectedOpened bool
	}{
		{
			name: "server members",
			context: UIHintContext{
				Command: "server members",
			},
			expectedURL: "http://127.0.0.1:4646/ui/servers",
		},
		{
			name: "node status (many)",
			context: UIHintContext{
				Command: "node status",
			},
			expectedURL: "http://127.0.0.1:4646/ui/clients",
		},
		{
			name: "node status (single)",
			context: UIHintContext{
				Command: "node status single",
				PathParams: map[string]string{
					"nodeID": "node-1",
				},
			},
			expectedURL: "http://127.0.0.1:4646/ui/clients/node-1",
		},
		{
			name: "job status (many)",
			context: UIHintContext{
				Command: "job status",
			},
			expectedURL: "http://127.0.0.1:4646/ui/jobs",
		},
		{
			name: "job status (single)",
			context: UIHintContext{
				Command: "job status single",
				PathParams: map[string]string{
					"jobID":     "example-job",
					"namespace": "default",
				},
			},
			expectedURL: "http://127.0.0.1:4646/ui/jobs/example-job@default",
		},
		{
			name: "job run (default ns)",
			context: UIHintContext{
				Command: "job run",
				PathParams: map[string]string{
					"jobID":     "example-job",
					"namespace": "default",
				},
			},
			expectedURL: "http://127.0.0.1:4646/ui/jobs/example-job@default",
		},
		{
			name: "job run (non-default ns)",
			context: UIHintContext{
				Command: "job run",
				PathParams: map[string]string{
					"jobID":     "example-job",
					"namespace": "prod",
				},
			},
			expectedURL: "http://127.0.0.1:4646/ui/jobs/example-job@prod",
		},
		{
			name: "job dispatch (default ns)",
			context: UIHintContext{
				Command: "job dispatch",
				PathParams: map[string]string{
					"dispatchID": "dispatch-1",
					"namespace":  "default",
				},
			},
			expectedURL: "http://127.0.0.1:4646/ui/jobs/dispatch-1@default",
		},
		{
			name: "job dispatch (non-default ns)",
			context: UIHintContext{
				Command: "job dispatch",
				PathParams: map[string]string{
					"dispatchID": "dispatch-1",
					"namespace":  "toronto",
				},
			},
			expectedURL: "http://127.0.0.1:4646/ui/jobs/dispatch-1@toronto",
		},
		{
			name: "eval list",
			context: UIHintContext{
				Command: "eval list",
			},
			expectedURL: "http://127.0.0.1:4646/ui/evaluations",
		},
		{
			name: "eval status",
			context: UIHintContext{
				Command: "eval status",
				PathParams: map[string]string{
					"evalID": "eval-1",
				},
			},
			expectedURL: "http://127.0.0.1:4646/ui/evaluations?currentEval=eval-1",
		},
		{
			name: "deployment status",
			context: UIHintContext{
				Command: "deployment status",
				PathParams: map[string]string{
					"jobID": "example-job",
				},
			},
			expectedURL: "http://127.0.0.1:4646/ui/jobs/example-job/deployments",
		},
		{
			name: "var list (root)",
			context: UIHintContext{
				Command: "var list",
			},
			expectedURL: "http://127.0.0.1:4646/ui/variables",
		},
		{
			name: "var list (path)",
			context: UIHintContext{
				Command: "var list prefix",
				PathParams: map[string]string{
					"prefix": "foo",
				},
			},
			expectedURL: "http://127.0.0.1:4646/ui/variables/path/foo",
		},
		{
			name: "var get",
			context: UIHintContext{
				Command: "var get",
				PathParams: map[string]string{
					"path":      "foo",
					"namespace": "default",
				},
			},
			expectedURL: "http://127.0.0.1:4646/ui/variables/var/foo@default",
		},
		{
			name: "var put",
			context: UIHintContext{
				Command: "var put",
				PathParams: map[string]string{
					"path":      "foo",
					"namespace": "default",
				},
			},
			expectedURL: "http://127.0.0.1:4646/ui/variables/var/foo@default",
		},
		{
			name: "alloc status",
			context: UIHintContext{
				Command: "alloc status",
				PathParams: map[string]string{
					"allocID": "alloc-1",
				},
			},
			expectedURL: "http://127.0.0.1:4646/ui/allocations/alloc-1",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			route := CommandUIRoutes[tc.context.Command]
			expectedHint := fmt.Sprintf("\n\n==> %s in the Web UI: %s", route.Description, tc.expectedURL)

			m := &Meta{
				Ui: cli.NewMockUi(),
			}

			hint, err := m.showUIPath(tc.context)
			must.NoError(t, err)
			must.Eq(t, expectedHint, hint)
		})
	}

	// TODO: invalid/edge cases
	// - unknown command
	// - missing required params
	// - invalid path params
	// - --output flag on job run

	// TODO: browser opening tests
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"flag"
	"os"
	"reflect"
	"sort"
	"testing"

	"github.com/creack/pty"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
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
			require.NoError(t, err)
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

			require.Equal(t, !tc.ExpectColor, m.Colorize().Disable)
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

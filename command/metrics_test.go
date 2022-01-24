package command

import (
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

var _ cli.Command = &OperatorMetricsCommand{}

func TestCommand_Metrics_Cases(t *testing.T) {
	t.Parallel()

	srv, _, url := testServer(t, false, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &OperatorMetricsCommand{Meta: Meta{Ui: ui}}

	cases := []struct {
		name           string
		args           []string
		expectedCode   int
		expectedOutput string
		expectedError  string
	}{
		{
			"gotemplate MetricsSummary",
			[]string{"-address=" + url, "-t", "'{{ .Timestamp }}'"},
			0,
			"UTC",
			"",
		},
		{
			"json formatted MetricsSummary",
			[]string{"-address=" + url, "-json"},
			0,
			"{",
			"",
		},
		{
			"pretty print json",
			[]string{"-address=" + url, "-pretty"},
			0,
			"{",
			"",
		},
		{
			"prometheus format",
			[]string{"-address=" + url, "-format", "prometheus"},
			0,
			"# HELP",
			"",
		},
		{
			"bad argument",
			[]string{"-address=" + url, "-foo", "bar"},
			1,
			"Usage: nomad operator metrics",
			"flag provided but not defined: -foo",
		},
		{
			"bad address - no protocol",
			[]string{"-address=foo"},
			1,
			"",
			"Error getting metrics: Get \"/v1/metrics\": unsupported protocol scheme",
		},
		{
			"bad address - fake host",
			[]string{"-address=http://foo"},
			1,
			"",
			"dial tcp: lookup foo: Temporary failure in name resolution",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			code := cmd.Run(c.args)
			out := ui.OutputWriter.String()
			outerr := ui.ErrorWriter.String()

			require.Equalf(t, code, c.expectedCode, "expected exit code %d, got: %d: %s", c.expectedCode, code, outerr)
			require.Contains(t, out, c.expectedOutput, "expected output \"%s\", got \"%s\"", c.expectedOutput, out)
			require.Containsf(t, outerr, c.expectedError, "expected error \"%s\", got \"%s\"", c.expectedError, outerr)

			ui.OutputWriter.Reset()
			ui.ErrorWriter.Reset()
		})
	}
}

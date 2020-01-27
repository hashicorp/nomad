package command

import (
	"strings"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestQuotaApplyCommand_Implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &QuotaApplyCommand{}
}

func TestQuotaApplyCommand_Fails(t *testing.T) {
	t.Parallel()
	ui := new(cli.MockUi)
	cmd := &QuotaApplyCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	if code := cmd.Run([]string{"some", "bad", "args"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, commandErrorText(cmd)) {
		t.Fatalf("expected help output, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	if code := cmd.Run([]string{"-address=nope"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, commandErrorText(cmd)) {
		t.Fatalf("name required error, got: %s", out)
	}
	ui.ErrorWriter.Reset()
}

func TestQuotaApplyNetwork(t *testing.T) {
	t.Parallel()

	mbits := 20

	cases := []struct {
		hcl string
		q   *api.QuotaSpec
		err string
	}{{
		hcl: `limit {region = "global", region_limit {network {mbits = 20}}}`,
		q: &api.QuotaSpec{
			Limits: []*api.QuotaLimit{{
				Region: "global",
				RegionLimit: &api.Resources{
					Networks: []*api.NetworkResource{{
						MBits: &mbits,
					}},
				},
			}},
		},
		err: "",
	}, {
		hcl: `limit {region = "global", region_limit {network { mbits = 20, device = "eth0"}}}`,
		q:   nil,
		err: "network -> invalid key: device",
	}}

	for _, c := range cases {
		t.Run(c.hcl, func(t *testing.T) {
			q, err := parseQuotaSpec([]byte(c.hcl))
			require.Equal(t, c.q, q)
			if c.err != "" {
				require.Contains(t, err.Error(), c.err)
			}
		})
	}
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"strings"
	"testing"

	"github.com/hashicorp/cli"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/shoenig/test/must"
)

func TestQuotaApplyCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &QuotaApplyCommand{}
}

func TestQuotaApplyCommand_Fails(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
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

func TestQuotaParse(t *testing.T) {

	in := []byte(`
name        = "default-quota"
description = "Limit the shared default namespace"

limit {
  region = "global"
  region_limit {
    cores      = 0
    cpu        = 2500
    memory     = 1000
    memory_max = 1000
    device "nvidia/gpu/1080ti" {
      count = 1
    }
    storage {
      variables    = 1000   # in MB
      host_volumes = "100 GiB"
    }
  }
}
`)

	spec, err := parseQuotaSpec(in)
	must.NoError(t, err)

	must.Eq(t, &api.QuotaSpec{
		Name:        "default-quota",
		Description: "Limit the shared default namespace",
		Limits: []*api.QuotaLimit{{
			Region: "global",
			RegionLimit: &api.QuotaResources{
				CPU:         pointer.Of(2500),
				Cores:       pointer.Of(0),
				MemoryMB:    pointer.Of(1000),
				MemoryMaxMB: pointer.Of(1000),
				Devices: []*api.RequestedDevice{{
					Name:  "nvidia/gpu/1080ti",
					Count: pointer.Of(uint64(1)),
				}},
				Storage: &api.QuotaStorageResources{
					VariablesMB:   1000,
					HostVolumesMB: 102_400,
				},
			},
		}},
	}, spec)
}

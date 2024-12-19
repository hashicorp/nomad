// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package cni

import (
	"testing"

	"github.com/hashicorp/nomad/e2e/v3/cluster3"
	"github.com/hashicorp/nomad/e2e/v3/jobs3"
	"github.com/shoenig/test/must"
)

func TestCNIIntegration(t *testing.T) {
	cluster3.Establish(t,
		cluster3.Leader(),
		cluster3.LinuxClients(1),
	)

	t.Run("testConfigFormats", testConfigFormats)
	t.Run("testArgs", testCNIArgs)
}

func testConfigFormats(t *testing.T) {
	for _, format := range []string{
		"conflist",
		"conf",
		"json",
	} {
		t.Run(format, func(t *testing.T) {
			// if this does not error, config must be good.
			jobs3.Submit(t, "./input/config_format.nomad.hcl",
				jobs3.Var("format", format),
			)
		})
	}
}

func testCNIArgs(t *testing.T) {
	job, _ := jobs3.Submit(t, "./input/cni_args.nomad.hcl")
	logs := job.Exec("group", "task", []string{"cat", "local/victory"})
	t.Logf("FancyMessage: %s", logs.Stdout)
	// "default" is the Nomad node's pool, interpolated in the jobspec, passed
	// through the CNI plugin, and cat-ed by the task.
	must.Eq(t, "default\n", logs.Stdout)
}

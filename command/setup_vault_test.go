// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/helper/pointer"
)

func TestSetupVaultCommand_Run(t *testing.T) {
	ci.Parallel(t)

	// Start in dev mode so we get a node registration
	srv, client, url := testServer(t, true, func(c *agent.Config) {
		c.DevMode = true
		c.Vaults[0].Name = "default"
		c.Vaults[0].Enabled = pointer.Of(true)
	})
	defer srv.Shutdown()

	// Register a job with a vault block but without an identity for Vault.
	job := testJob("test")
	job.TaskGroups[0].Tasks[0].Vault = &api.Vault{
		Cluster:  "default",
		Policies: []string{"test"},
	}
	_, _, err := client.Jobs().Register(job, nil)
	must.NoError(t, err)

	job, _, err = client.Jobs().Info(*job.ID, nil)
	must.NoError(t, err)

	testCases := []struct {
		name        string
		args        []string
		expectedErr string
		expectedRC  int
		expectedOut string
	}{
		{
			name: "-check flags",
			args: []string{
				"-json",
				"-t", "{{.}}",
				"-verbose",
			},
			expectedRC:  1,
			expectedErr: "The -json, -verbose, and -t options can only be used with -check",
		},
		{
			name: "-check",
			args: []string{
				"-check",
				"-address", url,
			},
			expectedRC: 0,
			expectedOut: `
Jobs Without Workload Identity for Vault
The following jobs access Vault but are not configured for workload identity.

You should redeploy them before fully migrating to workload identities with
Vault to prevent unexpected errors if their tokens need to be recreated.

Refer to https://developer.hashicorp.com/nomad/s/vault-workload-identity-migration
for more information.

ID    Namespace  Type   Status
test  default    batch  pending
`,
		},
		{
			name: "-check with -json",
			args: []string{
				"-check",
				"-json",
				"-address", url,
			},
			expectedRC: 0,
			expectedOut: fmt.Sprintf(`{
    "JobsWithoutVaultIdentity": [
        {
            "CreateIndex": 10,
            "Datacenters": [
                "dc1"
            ],
            "ID": "test",
            "JobModifyIndex": %d,
            "JobSummary": null,
            "ModifyIndex": %d,
            "Name": "test",
            "Namespace": "default",
            "ParameterizedJob": false,
            "ParentID": "",
            "Periodic": false,
            "Priority": 1,
            "Status": "pending",
            "StatusDescription": "",
            "Stop": false,
            "SubmitTime": %d,
            "Type": "batch"
        }
    ],
    "OutdatedNodes": [],
    "VaultTokens": []
}
`, *job.CreateIndex, *job.ModifyIndex, *job.SubmitTime),
		},
		{
			name: "-check with -t",
			args: []string{
				"-check",
				"-t", "{{with index .JobsWithoutVaultIdentity 0}}{{.ID}}{{end}}",
				"-address", url,
			},
			expectedRC:  0,
			expectedOut: "test\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ui := cli.NewMockUi()
			meta := Meta{Ui: ui}

			defer func() {
				if t.Failed() {
					fmt.Println(ui.ErrorWriter.String())
					fmt.Println(ui.OutputWriter.String())
				}
			}()

			cmd := &SetupVaultCommand{Meta: meta}
			got := cmd.Run(tc.args)
			must.Eq(t, tc.expectedRC, got)

			if tc.expectedErr != "" {
				must.StrContains(t, ui.ErrorWriter.String(), tc.expectedErr)
			} else {
				must.Eq(t, ui.ErrorWriter.String(), "")
				must.Eq(t, ui.OutputWriter.String(), tc.expectedOut)
			}
		})
	}
}

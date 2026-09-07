// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/cli"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/shoenig/test/must"
)

func TestSetupVaultCommand_renderVaultPolicy(t *testing.T) {
	ci.Parallel(t)

	const accessor = "auth_jwt_0a1b2c3d"
	policyBody := string(vaultPolicyBody)

	// The default "secret" mount renders identically to substituting only the
	// accessor, so omitting -kv-path leaves the policy byte-for-byte unchanged.
	must.Eq(t,
		strings.ReplaceAll(policyBody, "auth_jwt_X", accessor),
		renderVaultPolicy(policyBody, accessor, "secret"),
	)

	testCases := []struct {
		name   string
		kvPath string
		want   string
	}{
		{name: "custom mount", kvPath: "kv", want: "kv/data/"},
		{name: "nested mount", kvPath: "kv/mongo", want: "kv/mongo/data/"},
		{name: "trailing slash trimmed", kvPath: "kv/", want: "kv/data/"},
		{name: "leading slash trimmed", kvPath: "/kv", want: `path "kv/data/`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := renderVaultPolicy(policyBody, accessor, tc.kvPath)
			must.StrContains(t, got, tc.want)
			must.StrContains(t, got, accessor)
			must.StrNotContains(t, got, "secret/")
			must.StrNotContains(t, got, "auth_jwt_X")
			// every path stanza survives the substitution
			must.Eq(t, strings.Count(policyBody, `path "`), strings.Count(got, `path "`))
		})
	}
}

func TestSetupVaultCommand_Run_emptyKVPath(t *testing.T) {
	ci.Parallel(t)

	for _, kvPath := range []string{"", "/", "///"} {
		ui := cli.NewMockUi()
		cmd := &SetupVaultCommand{Meta: Meta{Ui: ui}}
		rc := cmd.Run([]string{"-kv-path", kvPath})
		must.Eq(t, 1, rc)
		must.StrContains(t, ui.ErrorWriter.String(), "non-empty mount path")
	}
}

func TestSetupVaultCommand_Run(t *testing.T) {
	ci.Parallel(t)

	// Start in dev mode so we get a node registration
	srv, client, url := testServer(t, true, func(c *agent.Config) {
		c.DevMode = true
		c.Vaults[0].Name = "default"
		c.Vaults[0].Enabled = new(true)
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
            "CreateIndex": %d,
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
    "VaultTokens": null
}
`, *job.CreateIndex, *job.CreateIndex, *job.ModifyIndex, *job.SubmitTime),
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

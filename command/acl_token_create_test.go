// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/hashicorp/cli"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/shoenig/test/must"
)

func TestACLTokenCreateCommand(t *testing.T) {
	ci.Parallel(t)

	config := func(c *agent.Config) {
		c.ACL.Enabled = true
	}

	srv, _, url := testServer(t, true, config)
	defer srv.Shutdown()

	// Bootstrap an initial ACL token
	token := srv.RootToken
	must.NotNil(t, token)

	ui := cli.NewMockUi()
	cmd := &ACLTokenCreateCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Request to create a new token without providing a valid management token
	code := cmd.Run([]string{"-address=" + url, "-token=foo", "-policy=foo", "-type=client"})
	must.One(t, code)

	// Request to create a new token with a valid management token that does
	// not have an expiry set.
	code = cmd.Run([]string{"-address=" + url, "-token=" + token.SecretID, "-policy=foo", "-type=client"})
	must.Zero(t, code)

	// Check the output
	out := ui.OutputWriter.String()
	must.StrContains(t, out, "[foo]")
	must.StrContains(t, out, "Expiry Time  = <none>")

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	// Test with a no-expiry token and -json/-t flag
	testCasesNoTTL := []string{"-json", "-t='{{ .Policies }}'"}
	var jsonMap map[string]interface{}
	for _, outputFormatFlag := range testCasesNoTTL {
		code = cmd.Run([]string{"-address=" + url, "-token=" + token.SecretID, "-policy=foo", "-type=client", outputFormatFlag})
		must.Zero(t, code)

		// Check the output
		out = ui.OutputWriter.String()
		must.StrContains(t, out, "foo")
		if outputFormatFlag == "-json" {
			err := json.Unmarshal([]byte(out), &jsonMap)
			must.NoError(t, err)
		}

		ui.OutputWriter.Reset()
		ui.ErrorWriter.Reset()
	}

	// Create a new token that has an expiry TTL set and check the response.
	code = cmd.Run([]string{"-address=" + url, "-token=" + token.SecretID, "-type=management", "-ttl=10m"})
	must.Zero(t, code)

	out = ui.OutputWriter.String()
	must.StrNotContains(t, out, "Expiry Time  = <none>")
	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	// Test with a token that has expiry TTL set and -json/-t flag
	testCasesWithTTL := [][]string{{"-json", "ExpirationTTL"}, {"-t='{{ .ExpirationTTL }}'", "10m0s"}}
	for _, outputFormatFlag := range testCasesWithTTL {
		code = cmd.Run([]string{"-address=" + url, "-token=" + token.SecretID, "-type=management", "-ttl=10m", outputFormatFlag[0]})
		must.Zero(t, code)

		// Check the output
		out = ui.OutputWriter.String()
		if outputFormatFlag[0] == "-json" {
			err := json.Unmarshal([]byte(out), &jsonMap)
			must.NoError(t, err)
		}
		must.StrContains(t, out, outputFormatFlag[1])
		ui.OutputWriter.Reset()
		ui.ErrorWriter.Reset()
	}
}

func TestACLTokenCreateCommand_UploadFromFile(t *testing.T) {
	ci.Parallel(t)

	config := func(c *agent.Config) {
		c.ACL.Enabled = true
	}

	srv, _, url := testServer(t, true, config)
	defer srv.Shutdown()

	token := srv.RootToken
	must.NotNil(t, token)

	ui := cli.NewMockUi()
	cmd := &ACLTokenCreateCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	accessorID := "b306571d-a3fa-42d2-ac5b-bbe49d8c3c7f"
	secretID := "c28ba0f6-ab8d-4e4d-b9f0-7f9f5d1c2e3a"

	// Write the SecretID to a temp file
	secretFile := t.TempDir() + "/secret.txt"
	must.NoError(t, os.WriteFile(secretFile, []byte(secretID), 0600))

	testCases := []struct {
		desc        string
		args        []string
		expectedErr bool
		errorMsg    string
	}{
		{
			desc:        "providing -accessor without a SecretID file should fail",
			args:        []string{"-address=" + url, "-token=" + token.SecretID, "-type=client", "-policy=foo", "-accessor=" + accessorID},
			expectedErr: true,
			errorMsg:    "-accessor requires a SecretID",
		},
		{
			desc:        "providing a SecretID file without -accessor should fail",
			args:        []string{"-address=" + url, "-token=" + token.SecretID, "-type=client", "-policy=foo", secretFile},
			expectedErr: true,
			errorMsg:    "-accessor was not set",
		},
		{
			desc:        "successfully upload a client token with pre-specified IDs",
			args:        []string{"-address=" + url, "-token=" + token.SecretID, "-type=client", "-policy=foo", "-accessor=" + accessorID, secretFile},
			expectedErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			code := cmd.Run(tc.args)
			if tc.expectedErr {
				must.One(t, code)
				must.StrContains(t, ui.ErrorWriter.String(), tc.errorMsg)
			} else {
				must.Zero(t, code)
			}
			ui.OutputWriter.Reset()
			ui.ErrorWriter.Reset()
		})
	}
}

func TestACLTokenCreateCommand_UploadFromStdin(t *testing.T) {
	config := func(c *agent.Config) {
		c.ACL.Enabled = true
	}

	srv, _, url := testServer(t, true, config)
	defer srv.Shutdown()

	token := srv.RootToken
	must.NotNil(t, token)

	ui := cli.NewMockUi()
	cmd := &ACLTokenCreateCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	accessorID := "fe7d9be4-4419-4c3a-b942-7e616c7f6f1f"
	secretID := "2a1c0e84-0db7-45c7-bec0-5b9f3a0e0b49"

	fakeStdin, err := os.CreateTemp("", "nomad-acl-token-secret")
	must.NoError(t, err)
	defer os.Remove(fakeStdin.Name())
	defer fakeStdin.Close()

	_, err = fakeStdin.WriteString(secretID + "\n")
	must.NoError(t, err)
	_, err = fakeStdin.Seek(0, 0)
	must.NoError(t, err)

	oldStdin := os.Stdin
	os.Stdin = fakeStdin
	defer func() { os.Stdin = oldStdin }()

	code := cmd.Run([]string{
		"-address=" + url,
		"-token=" + token.SecretID,
		"-type=client",
		"-policy=foo",
		"-accessor=" + accessorID,
		"-",
	})
	must.Zero(t, code)
	must.StrContains(t, ui.OutputWriter.String(), accessorID)
	must.StrContains(t, ui.OutputWriter.String(), secretID)
}

func Test_generateACLTokenRoleLinks(t *testing.T) {
	ci.Parallel(t)

	inputRoleNames := []string{
		"duplicate",
		"policy1",
		"policy2",
		"duplicate",
	}
	inputRoleIDs := []string{
		"77a780d8-2dee-7c7f-7822-6f5471c5cbb2",
		"56850b06-a343-a772-1a5c-ad083fd8a50e",
		"77a780d8-2dee-7c7f-7822-6f5471c5cbb2",
		"77a780d8-2dee-7c7f-7822-6f5471c5cbb2",
	}
	expectedOutput := []*api.ACLTokenRoleLink{
		{Name: "duplicate"},
		{Name: "policy1"},
		{Name: "policy2"},
		{ID: "77a780d8-2dee-7c7f-7822-6f5471c5cbb2"},
		{ID: "56850b06-a343-a772-1a5c-ad083fd8a50e"},
	}
	must.SliceContainsAll(t, generateACLTokenRoleLinks(inputRoleNames, inputRoleIDs), expectedOutput)
}

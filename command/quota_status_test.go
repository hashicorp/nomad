// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build ent
// +build ent

package command

import (
	"encoding/json"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
	"github.com/shoenig/test/must"
)

func TestQuotaStatusCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &QuotaStatusCommand{}
}

func TestQuotaStatusCommand_Fails(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &QuotaStatusCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	code := cmd.Run([]string{"some", "bad", "args"})
	must.One(t, code)

	out := ui.ErrorWriter.String()
	must.StrContains(t, out, commandErrorText(cmd))
	ui.ErrorWriter.Reset()

	code = cmd.Run([]string{"-address=nope", "foo"})
	must.One(t, code)

	out = ui.ErrorWriter.String()
	must.StrContains(t, out, "retrieving quota")

	ui.ErrorWriter.Reset()
}

func TestQuotaStatusCommand_Run(t *testing.T) {
	ci.Parallel(t)

	// Create a server
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &QuotaStatusCommand{Meta: Meta{Ui: ui}}

	// Create a quota to delete
	qs := testQuotaSpec()
	_, err := client.Quotas().Register(qs, nil)
	must.NoError(t, err)

	// Delete a namespace
	code := cmd.Run([]string{"-address=" + url, qs.Name})
	must.Zero(t, code)

	// Check for basic spec
	out := ui.OutputWriter.String()
	must.StrContains(t, out, "= quota-test-")

	// Check for usage
	must.StrContains(t, out, "0 / 100")

	ui.OutputWriter.Reset()

	// List json
	code = cmd.Run([]string{"-address=" + url, "-json", qs.Name})
	must.Zero(t, code)

	outJson := api.QuotaSpec{}
	err = json.Unmarshal(ui.OutputWriter.Bytes(), &outJson)
	must.NoError(t, err)

	ui.OutputWriter.Reset()

	// Go template to format the output
	code = cmd.Run([]string{"-address=" + url, "-t", "{{ .Name }}", qs.Name})
	must.Zero(t, code)

	out = ui.OutputWriter.String()
	must.StrContains(t, out, "test")

	ui.OutputWriter.Reset()
}

func TestQuotaStatusCommand_AutocompleteArgs(t *testing.T) {
	ci.Parallel(t)

	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &QuotaStatusCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Create a quota
	qs := testQuotaSpec()
	_, err := client.Quotas().Register(qs, nil)
	must.NoError(t, err)

	args := complete.Args{Last: "quot"}
	predictor := cmd.AutocompleteArgs()

	res := predictor.Predict(args)
	must.One(t, len(res))
	must.StrContains(t, qs.Name, res[0])
}

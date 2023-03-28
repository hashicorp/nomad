//go:build ent
// +build ent

package command

import (
	"encoding/json"
	"strings"
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
	if code := cmd.Run([]string{"some", "bad", "args"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, commandErrorText(cmd)) {
		t.Fatalf("expected help output, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	if code := cmd.Run([]string{"-address=nope", "foo"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "retrieving quota") {
		t.Fatalf("connection error, got: %s", out)
	}
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
	if code := cmd.Run([]string{"-address=" + url, qs.Name}); code != 0 {
		t.Fatalf("expected exit 0, got: %d; %v", code, ui.ErrorWriter.String())
	}

	// Check for basic spec
	out := ui.OutputWriter.String()
	must.StrContains(t, out, "= test")

	// Check for usage
	must.StrContains(t, out, "0 / 100")

	ui.OutputWriter.Reset()

	// List json
	must.Zero(t, cmd.Run([]string{"-address=" + url, "-json", qs.Name}))

	outJson := api.QuotaSpec{}
	err = json.Unmarshal(ui.OutputWriter.Bytes(), &outJson)
	must.NoError(t, err)

	ui.OutputWriter.Reset()

	// Go template to format the output
	code := cmd.Run([]string{"-address=" + url, "-t", "{{range .}}{{ .Name }}{{end}}", qs.Name})
	must.Zero(t, code)

	out = ui.OutputWriter.String()
	must.StrContains(t, out, "test-quota")

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

	args := complete.Args{Last: "t"}
	predictor := cmd.AutocompleteArgs()

	res := predictor.Predict(args)
	must.One(t, len(res))
	must.StrContains(t, qs.Name, res[0])
}

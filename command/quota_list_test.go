// +build ent

package command

import (
	"strings"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/assert"
)

func TestQuotaListCommand_Implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &QuotaListCommand{}
}

func TestQuotaListCommand_Fails(t *testing.T) {
	t.Parallel()
	ui := new(cli.MockUi)
	cmd := &QuotaListCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	if code := cmd.Run([]string{"some", "bad", "args"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, cmd.Help()) {
		t.Fatalf("expected help output, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	if code := cmd.Run([]string{"-address=nope"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Error retrieving quotas") {
		t.Fatalf("expected failed query error, got: %s", out)
	}
	ui.ErrorWriter.Reset()
}

func TestQuotaListCommand_List(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	// Create a server
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := new(cli.MockUi)
	cmd := &QuotaListCommand{Meta: Meta{Ui: ui}}

	// Create a quota
	qs := testQuotaSpec()
	_, err := client.Quotas().Register(qs, nil)
	assert.Nil(err)

	// List should contain the new quota
	if code := cmd.Run([]string{"-address=" + url}); code != 0 {
		t.Fatalf("expected exit 0, got: %d; %v", code, ui.ErrorWriter.String())
	}
	out := ui.OutputWriter.String()
	if !strings.Contains(out, qs.Name) || !strings.Contains(out, qs.Description) {
		t.Fatalf("expected quota, got: %s", out)
	}
	ui.OutputWriter.Reset()

	// List json
	t.Log(url)
	if code := cmd.Run([]string{"-address=" + url, "-json"}); code != 0 {
		t.Fatalf("expected exit 0, got: %d; %v", code, ui.ErrorWriter.String())
	}
	out = ui.OutputWriter.String()
	if !strings.Contains(out, "CreateIndex") {
		t.Fatalf("expected json output, got: %s", out)
	}
	ui.OutputWriter.Reset()
}

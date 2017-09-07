// +build pro ent

package command

import (
	"strings"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/assert"
)

func TestNamespaceApplyCommand_Implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &NamespaceApplyCommand{}
}

func TestNamespaceApplyCommand_Fails(t *testing.T) {
	t.Parallel()
	ui := new(cli.MockUi)
	cmd := &NamespaceApplyCommand{Meta: Meta{Ui: ui}}

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
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "name required") {
		t.Fatalf("name required error, got: %s", out)
	}
	ui.ErrorWriter.Reset()
}

func TestNamespaceApplyCommand_Good(t *testing.T) {
	t.Parallel()

	// Create a server
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := new(cli.MockUi)
	cmd := &NamespaceApplyCommand{Meta: Meta{Ui: ui}}

	// Create a namespace
	name, desc := "foo", "bar"
	if code := cmd.Run([]string{"-address=" + url, "-name=" + name, "-description=" + desc}); code != 0 {
		t.Fatalf("expected exit 0, got: %d; %v", code, ui.ErrorWriter.String())
	}

	namespaces, _, err := client.Namespaces().List(nil)
	assert.Nil(t, err)
	assert.Len(t, namespaces, 2)
}

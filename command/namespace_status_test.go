// +build ent

package command

import (
	"strings"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
	"github.com/stretchr/testify/assert"
)

func TestNamespaceStatusCommand_Implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &NamespaceStatusCommand{}
}

func TestNamespaceStatusCommand_Fails(t *testing.T) {
	t.Parallel()
	ui := new(cli.MockUi)
	cmd := &NamespaceStatusCommand{Meta: Meta{Ui: ui}}

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
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "retrieving namespace") {
		t.Fatalf("connection error, got: %s", out)
	}
	ui.ErrorWriter.Reset()
}

func TestNamespaceStatusCommand_Good(t *testing.T) {
	t.Parallel()

	// Create a server
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := new(cli.MockUi)
	cmd := &NamespaceStatusCommand{Meta: Meta{Ui: ui}}

	// Create a namespace
	ns := &api.Namespace{
		Name: "foo",
	}
	_, err := client.Namespaces().Register(ns, nil)
	assert.Nil(t, err)

	// Check status on namespace
	if code := cmd.Run([]string{"-address=" + url, ns.Name}); code != 0 {
		t.Fatalf("expected exit 0, got: %d; %v", code, ui.ErrorWriter.String())
	}

	// Check for basic spec
	out := ui.OutputWriter.String()
	if !strings.Contains(out, "= foo") {
		t.Fatalf("expected quota, got: %s", out)
	}
}

func TestNamespaceStatusCommand_Good_Quota(t *testing.T) {
	t.Parallel()

	// Create a server
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := new(cli.MockUi)
	cmd := &NamespaceStatusCommand{Meta: Meta{Ui: ui}}

	// Create a quota to delete
	qs := testQuotaSpec()
	_, err := client.Quotas().Register(qs, nil)
	assert.Nil(t, err)

	// Create a namespace
	ns := &api.Namespace{
		Name:  "foo",
		Quota: qs.Name,
	}
	_, err = client.Namespaces().Register(ns, nil)
	assert.Nil(t, err)

	// Check status on namespace
	if code := cmd.Run([]string{"-address=" + url, ns.Name}); code != 0 {
		t.Fatalf("expected exit 0, got: %d; %v", code, ui.ErrorWriter.String())
	}

	// Check for basic spec
	out := ui.OutputWriter.String()
	if !strings.Contains(out, "= foo") {
		t.Fatalf("expected quota, got: %s", out)
	}

	// Check for usage
	if !strings.Contains(out, "0 / 100") {
		t.Fatalf("expected quota, got: %s", out)
	}
}

func TestNamespaceStatusCommand_AutocompleteArgs(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()

	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := new(cli.MockUi)
	cmd := &NamespaceStatusCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Create a namespace
	ns := &api.Namespace{
		Name: "foo",
	}
	_, err := client.Namespaces().Register(ns, nil)
	assert.Nil(err)

	args := complete.Args{Last: "f"}
	predictor := cmd.AutocompleteArgs()

	res := predictor.Predict(args)
	assert.Equal(1, len(res))
	assert.Equal(ns.Name, res[0])
}

// This test should demonstrate the behavior of a namespace
// and prefix collision.  In that case, the Namespace status
// command should pull the matching namespace rather than
// displaying the multiple match error
func TestNamespaceStatusCommand_NamespaceMatchesPrefix(t *testing.T) {
	t.Parallel()

	// Create a server
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := new(cli.MockUi)
	cmd := &NamespaceStatusCommand{Meta: Meta{Ui: ui}}

	// Create a namespace that uses foo as a prefix
	ns := &api.Namespace{Name: "fooBar"}
	_, err := client.Namespaces().Register(ns, nil)
	assert.Nil(t, err)

	// Create a foo namespace
	ns2 := &api.Namespace{Name: "foo"}
	_, err = client.Namespaces().Register(ns2, nil)
	assert.Nil(t, err)

	// Adding a NS after to prevent sort from creating
	// false successes
	ns = &api.Namespace{Name: "fooBaz"}
	_, err = client.Namespaces().Register(ns, nil)
	assert.Nil(t, err)

	// Check status on namespace
	code := cmd.Run([]string{"-address=" + url, ns2.Name})
	if code != 0 {
		t.Fatalf("expected exit 0, got: %d; %v", code, ui.ErrorWriter.String())
	}
	// Check to ensure we got the proper foo
	out := ui.OutputWriter.String()
	if !strings.Contains(out, "= foo\n") {
		t.Fatalf("expected namespace foo, got: %s", out)
	}
}

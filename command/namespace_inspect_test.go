// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"strings"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
	"github.com/shoenig/test/must"
)

func TestNamespaceInspectCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &NamespaceInspectCommand{}
}

func TestNamespaceInspectCommand_Fails(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &NamespaceInspectCommand{Meta: Meta{Ui: ui}}

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

func TestNamespaceInspectCommand_Good(t *testing.T) {
	ci.Parallel(t)

	// Create a server
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &NamespaceInspectCommand{Meta: Meta{Ui: ui}}

	// Create a namespace
	ns := &api.Namespace{
		Name: "foo",
	}
	_, err := client.Namespaces().Register(ns, nil)
	must.NoError(t, err)

	// Inspect
	if code := cmd.Run([]string{"-address=" + url, ns.Name}); code != 0 {
		t.Fatalf("expected exit 0, got: %d; %v", code, ui.ErrorWriter.String())
	}

	out := ui.OutputWriter.String()
	if !strings.Contains(out, ns.Name) {
		t.Fatalf("expected namespace, got: %s", out)
	}
}

func TestNamespaceInspectCommand_AutocompleteArgs(t *testing.T) {
	ci.Parallel(t)

	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &NamespaceInspectCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Create a namespace
	ns := &api.Namespace{
		Name: "foo",
	}
	_, err := client.Namespaces().Register(ns, nil)
	must.NoError(t, err)

	args := complete.Args{Last: "f"}
	predictor := cmd.AutocompleteArgs()

	res := predictor.Predict(args)
	must.Len(t, 1, res)
	must.Eq(t, ns.Name, res[0])
}

// This test should demonstrate the behavior of a namespace
// and prefix collision.  In that case, the Namespace status
// command should pull the matching namespace rather than
// displaying the multiple match error
func TestNamespaceInspectCommand_NamespaceMatchesPrefix(t *testing.T) {
	ci.Parallel(t)

	// Create a server
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &NamespaceInspectCommand{Meta: Meta{Ui: ui}}

	// Create a namespace that uses foo as a prefix
	ns := &api.Namespace{Name: "fooBar"}
	_, err := client.Namespaces().Register(ns, nil)
	must.NoError(t, err)

	// Create a foo namespace
	ns2 := &api.Namespace{Name: "foo"}
	_, err = client.Namespaces().Register(ns2, nil)
	must.NoError(t, err)

	// Adding a NS after to prevent sort from creating
	// false successes
	ns = &api.Namespace{Name: "fooBaz"}
	_, err = client.Namespaces().Register(ns, nil)
	must.NoError(t, err)

	// Check status on namespace
	code := cmd.Run([]string{"-address=" + url, ns2.Name})
	must.Zero(t, code)

	// Check to ensure we got the proper foo
	out := ui.OutputWriter.String()
	must.StrContains(t, out, "\"foo\",\n")
}

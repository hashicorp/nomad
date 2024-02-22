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

func TestNamespaceDeleteCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &NamespaceDeleteCommand{}
}

func TestNamespaceDeleteCommand_Fails(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &NamespaceDeleteCommand{Meta: Meta{Ui: ui}}

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
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "deleting namespace") {
		t.Fatalf("connection error, got: %s", out)
	}
	ui.ErrorWriter.Reset()
}

func TestNamespaceDeleteCommand_Good(t *testing.T) {
	ci.Parallel(t)

	// Create a server
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &NamespaceDeleteCommand{Meta: Meta{Ui: ui}}

	// Create a namespace to delete
	ns := &api.Namespace{
		Name: "foo",
	}
	_, err := client.Namespaces().Register(ns, nil)
	must.NoError(t, err)

	// Delete a namespace
	if code := cmd.Run([]string{"-address=" + url, ns.Name}); code != 0 {
		t.Fatalf("expected exit 0, got: %d; %v", code, ui.ErrorWriter.String())
	}

	namespaces, _, err := client.Namespaces().List(nil)
	must.NoError(t, err)
	must.Len(t, 1, namespaces)
}

func TestNamespaceDeleteCommand_AutocompleteArgs(t *testing.T) {
	ci.Parallel(t)

	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &NamespaceDeleteCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Create a namespace other than default
	ns := &api.Namespace{
		Name: "diddo",
	}
	_, err := client.Namespaces().Register(ns, nil)
	must.NoError(t, err)

	args := complete.Args{Last: "d"}
	predictor := cmd.AutocompleteArgs()

	res := predictor.Predict(args)
	must.Len(t, 1, res)
	must.Eq(t, ns.Name, res[0])
}

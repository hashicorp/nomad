// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

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
	"github.com/stretchr/testify/assert"
)

func TestNamespaceStatusCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &NamespaceStatusCommand{}
}

func TestNamespaceStatusCommand_Fails(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &NamespaceStatusCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	code := cmd.Run([]string{"some", "bad", "args"})
	must.One(t, code)

	must.StrContains(t, ui.ErrorWriter.String(), commandErrorText(cmd))

	ui.ErrorWriter.Reset()

	code = cmd.Run([]string{"-address=nope", "foo"})
	must.One(t, code)

	must.StrContains(t, ui.ErrorWriter.String(), "retrieving namespace")
	ui.ErrorWriter.Reset()
}

func TestNamespaceStatusCommand_Run(t *testing.T) {
	ci.Parallel(t)

	// Create a server
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &NamespaceStatusCommand{Meta: Meta{Ui: ui}}

	// Create a namespace
	ns := &api.Namespace{
		Name: "foo",
	}
	_, err := client.Namespaces().Register(ns, nil)
	must.NoError(t, err)

	// Check status on namespace
	code := cmd.Run([]string{"-address=" + url, ns.Name})
	must.Zero(t, code)

	// Check for basic spec
	out := ui.OutputWriter.String()
	if !strings.Contains(out, "= foo") {
		t.Fatalf("expected quota, got: %s", out)
	}

	ui.OutputWriter.Reset()

	// List json
	code = cmd.Run([]string{"-address=" + url, "-json", ns.Name})
	must.Zero(t, code)

	outJson := api.Namespace{}
	err = json.Unmarshal(ui.OutputWriter.Bytes(), &outJson)
	must.NoError(t, err)

	ui.OutputWriter.Reset()

	// Go template to format the output
	code = cmd.Run([]string{"-address=" + url, "-t", "{{.Name}}", ns.Name})
	must.Zero(t, code)

	out = ui.OutputWriter.String()
	must.StrContains(t, out, "foo")

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()
}

func TestNamespaceStatusCommand_Run_Quota(t *testing.T) {
	ci.Parallel(t)

	// Create a server
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	if !srv.Enterprise {
		t.Skip("Skipping enterprise-only quota test")
	}

	ui := cli.NewMockUi()
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
	code := cmd.Run([]string{"-address=" + url, ns.Name})
	must.Zero(t, code)

	out := ui.OutputWriter.String()

	// Check for basic spec
	must.StrContains(t, out, "= foo")

	// Check for usage
	must.StrContains(t, out, "0 / 100")

}

func TestNamespaceStatusCommand_AutocompleteArgs(t *testing.T) {
	ci.Parallel(t)

	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &NamespaceStatusCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Create a namespace
	ns := &api.Namespace{
		Name: "foo",
	}
	_, err := client.Namespaces().Register(ns, nil)
	must.NoError(t, err)

	args := complete.Args{Last: "f"}
	predictor := cmd.AutocompleteArgs()

	res := predictor.Predict(args)
	must.One(t, len(res))
	must.StrContains(t, ns.Name, res[0])
}

// This test should demonstrate the behavior of a namespace
// and prefix collision.  In that case, the Namespace status
// command should pull the matching namespace rather than
// displaying the multiple match error
func TestNamespaceStatusCommand_NamespaceMatchesPrefix(t *testing.T) {
	ci.Parallel(t)

	// Create a server
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &NamespaceStatusCommand{Meta: Meta{Ui: ui}}

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
	must.StrContains(t, ui.OutputWriter.String(), "= foo\n")
}

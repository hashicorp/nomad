// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
	"github.com/shoenig/test/must"
)

func TestVarGetCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &VarGetCommand{}
}

func TestVarGetCommand_Fails(t *testing.T) {
	ci.Parallel(t)
	t.Run("bad_args", func(t *testing.T) {
		ci.Parallel(t)
		ui := cli.NewMockUi()
		cmd := &VarGetCommand{Meta: Meta{Ui: ui}}
		code := cmd.Run([]string{"some", "bad", "args"})
		out := ui.ErrorWriter.String()
		must.One(t, code)
		must.StrContains(t, out, commandErrorText(cmd))
	})
	t.Run("bad_address", func(t *testing.T) {
		ci.Parallel(t)
		ui := cli.NewMockUi()
		cmd := &VarGetCommand{Meta: Meta{Ui: ui}}
		code := cmd.Run([]string{"-address=nope", "foo"})
		must.One(t, code)
		must.StrContains(t, ui.ErrorWriter.String(), "retrieving variable")
		must.Eq(t, "", ui.OutputWriter.String())
	})
	t.Run("missing_template", func(t *testing.T) {
		ci.Parallel(t)
		ui := cli.NewMockUi()
		cmd := &VarGetCommand{Meta: Meta{Ui: ui}}
		code := cmd.Run([]string{`-out=go-template`, "foo"})
		out := strings.TrimSpace(ui.ErrorWriter.String())
		must.One(t, code)
		must.Eq(t, errMissingTemplate+"\n"+commandErrorText(cmd), out)
		must.Eq(t, "", ui.OutputWriter.String())
	})
	t.Run("unexpected_template", func(t *testing.T) {
		ci.Parallel(t)
		ui := cli.NewMockUi()
		cmd := &VarGetCommand{Meta: Meta{Ui: ui}}
		code := cmd.Run([]string{`-out=json`, `-template="bad"`, "foo"})
		out := strings.TrimSpace(ui.ErrorWriter.String())
		must.One(t, code)
		must.Eq(t, errUnexpectedTemplate+"\n"+commandErrorText(cmd), out)
		must.Eq(t, "", ui.OutputWriter.String())
	})
}

func TestVarGetCommand(t *testing.T) {
	ci.Parallel(t)

	// Create a server
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	testCases := []struct {
		name     string
		format   string
		template string
		expected string
		testPath string // defaulted to "test/var" in code; used for not-found
		exitCode int
		isError  bool
	}{
		{
			name:   "json",
			format: "json",
		},
		{
			name:   "table",
			format: "table",
		},
		{
			name:     "go-template",
			format:   "go-template",
			template: `{{.Namespace}}.{{.Path}}`,
			expected: "TestVarGetCommand-2-go-template.test/var",
		},
		{
			name:     "not-found",
			format:   "json",
			expected: errVariableNotFound,
			testPath: "not-found",
			isError:  true,
			exitCode: 1,
		},
	}
	for i, tc := range testCases {
		t.Run(fmt.Sprintf("%v_%s", i, tc.name), func(t *testing.T) {
			tc := tc
			ci.Parallel(t)
			var err error
			// Create a namespace for the test case
			testNS := strings.Map(validNS, t.Name())
			_, err = client.Namespaces().Register(&api.Namespace{Name: testNS}, nil)
			must.NoError(t, err)
			t.Cleanup(func() {
				_, _ = client.Namespaces().Delete(testNS, nil)
			})

			// Create a var to get
			sv := testVariable()
			sv.Namespace = testNS
			sv, _, err = client.Variables().Create(sv, nil)
			must.NoError(t, err)
			t.Cleanup(func() {
				_, _ = client.Variables().Delete(sv.Path, nil)
			})

			// Build and run the command
			ui := cli.NewMockUi()
			cmd := &VarGetCommand{Meta: Meta{Ui: ui}}
			args := []string{
				"-address=" + url,
				"-namespace=" + testNS,
				"-out=" + tc.format,
			}
			if tc.template != "" {
				args = append(args, "-template="+tc.template)
			}
			args = append(args, sv.Path)
			if tc.testPath != "" {
				// replace path with test case override
				args[len(args)-1] = tc.testPath
			}
			code := cmd.Run(args)

			// Check the output
			must.Eq(t, tc.exitCode, code)
			if tc.isError {
				must.Eq(t, tc.expected, strings.TrimSpace(ui.ErrorWriter.String()))
				return
			}
			switch tc.format {
			case "json":
				must.Eq(t, sv.AsPrettyJSON(), strings.TrimSpace(ui.OutputWriter.String()))
			case "table":
				out := ui.OutputWriter.String()
				outs := strings.Split(out, "\n")
				must.Len(t, 9, outs)
				must.Eq(t, "Namespace   = "+testNS, outs[0])
				must.Eq(t, "Path        = test/var", outs[1])
			case "go-template":
				must.Eq(t, tc.expected, strings.TrimSpace(ui.OutputWriter.String()))
			default:
				t.Fatalf("invalid format: %q", tc.format)
			}
		})
	}
	t.Run("Autocomplete", func(t *testing.T) {
		ci.Parallel(t)

		ui := cli.NewMockUi()
		cmd := &VarGetCommand{Meta: Meta{Ui: ui, flagAddress: url}}

		// Create a var
		testNS := strings.Map(validNS, t.Name())
		_, err := client.Namespaces().Register(&api.Namespace{Name: testNS}, nil)
		must.NoError(t, err)
		t.Cleanup(func() {
			_, _ = client.Namespaces().Delete(testNS, nil)
		})

		sv := testVariable()
		sv.Path = "special/variable"
		sv.Namespace = testNS
		sv, _, err = client.Variables().Create(sv, nil)
		must.NoError(t, err)
		t.Cleanup(func() {
			_, _ = client.Variables().Delete(sv.Path, nil)
		})

		args := complete.Args{Last: "s"}
		predictor := cmd.AutocompleteArgs()

		res := predictor.Predict(args)
		must.Len(t, 1, res)
		must.Eq(t, sv.Path, res[0])
	})
}

func validNS(r rune) rune {
	if r == '/' || r == '_' {
		return '-'
	}
	return r
}

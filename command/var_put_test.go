// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
)

func TestVarPutCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &VarPutCommand{}
}
func TestVarPutCommand_Fails(t *testing.T) {
	ci.Parallel(t)
	t.Run("bad_args", func(t *testing.T) {
		ci.Parallel(t)
		ui := cli.NewMockUi()
		cmd := &VarPutCommand{Meta: Meta{Ui: ui}}
		code := cmd.Run([]string{"-bad-flag"})
		out := ui.ErrorWriter.String()
		require.Equal(t, 1, code, "expected exit code 1, got: %d")
		require.Contains(t, out, commandErrorText(cmd), "expected help output, got: %s", out)
	})
	t.Run("bad_address", func(t *testing.T) {
		ci.Parallel(t)
		ui := cli.NewMockUi()
		cmd := &VarPutCommand{Meta: Meta{Ui: ui}}
		code := cmd.Run([]string{"-address=nope", "foo", "-"})
		out := ui.ErrorWriter.String()
		require.Equal(t, 1, code, "expected exit code 1, got: %d")
		require.Contains(t, out, "Error creating variable", "expected error creating variable, got: %s", out)
	})
	t.Run("missing_template", func(t *testing.T) {
		ci.Parallel(t)
		ui := cli.NewMockUi()
		cmd := &VarPutCommand{Meta: Meta{Ui: ui}}
		code := cmd.Run([]string{`-out=go-template`, "foo", "-"})
		out := strings.TrimSpace(ui.ErrorWriter.String())
		require.Equal(t, 1, code, "expected exit code 1, got: %d", code)
		require.Equal(t, errMissingTemplate+"\n"+commandErrorText(cmd), out)
	})
	t.Run("unexpected_template", func(t *testing.T) {
		ci.Parallel(t)
		ui := cli.NewMockUi()
		cmd := &VarPutCommand{Meta: Meta{Ui: ui}}
		code := cmd.Run([]string{`-out=json`, `-template="bad"`, "foo", "-"})
		out := strings.TrimSpace(ui.ErrorWriter.String())
		require.Equal(t, 1, code, "expected exit code 1, got: %d", code)
		require.Equal(t, errUnexpectedTemplate+"\n"+commandErrorText(cmd), out)
	})
	t.Run("bad_in", func(t *testing.T) {
		ci.Parallel(t)
		ui := cli.NewMockUi()
		cmd := &VarPutCommand{Meta: Meta{Ui: ui}}
		code := cmd.Run([]string{`-in=bad`, "foo", "-"})
		out := strings.TrimSpace(ui.ErrorWriter.String())
		require.Equal(t, 1, code, "expected exit code 1, got: %d", code)
		require.Equal(t, errInvalidInFormat+"\n"+commandErrorText(cmd), out)
	})
	t.Run("wildcard_namespace", func(t *testing.T) {
		ci.Parallel(t)
		ui := cli.NewMockUi()
		cmd := &VarPutCommand{Meta: Meta{Ui: ui}}
		code := cmd.Run([]string{`-namespace=*`, "foo", "-"})
		out := strings.TrimSpace(ui.ErrorWriter.String())
		require.Equal(t, 1, code, "expected exit code 1, got: %d", code)
		require.Equal(t, errWildcardNamespaceNotAllowed, out)
	})
}

func TestVarPutCommand_GoodJson(t *testing.T) {
	ci.Parallel(t)

	// Create a server
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &VarPutCommand{Meta: Meta{Ui: ui}}

	// Get the variable
	code := cmd.Run([]string{"-address=" + url, "-out=json", "test/var", "k1=v1", "k2=v2"})
	require.Equal(t, 0, code, "expected exit 0, got: %d; %v", code, ui.ErrorWriter.String())

	t.Cleanup(func() {
		_, _ = client.Variables().Delete("test/var", nil)
	})

	var outVar api.Variable
	b := ui.OutputWriter.Bytes()
	err := json.Unmarshal(b, &outVar)
	require.NoError(t, err, "error unmarshaling json: %v\nb: %s", err, b)
	require.Equal(t, "default", outVar.Namespace)
	require.Equal(t, "test/var", outVar.Path)
	require.Equal(t, api.VariableItems{"k1": "v1", "k2": "v2"}, outVar.Items)
}

func TestVarPutCommand_AutocompleteArgs(t *testing.T) {
	ci.Parallel(t)
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &VarPutCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Create a var
	sv := testVariable()
	_, _, err := client.Variables().Create(sv, nil)
	require.NoError(t, err)

	args := complete.Args{Last: "t"}
	predictor := cmd.AutocompleteArgs()

	res := predictor.Predict(args)
	require.Equal(t, 1, len(res))
	require.Equal(t, sv.Path, res[0])
}

func TestVarPutCommand_KeyWarning(t *testing.T) {
	// Extract invalid characters from warning message.
	r := regexp.MustCompile(`contains characters \[(.*)\]`)

	tcs := []struct {
		name     string
		goodKeys []string
		badKeys  []string
		badChars []string
	}{
		{
			name:     "simple",
			goodKeys: []string{"simple"},
		},
		{
			name:     "hasDot",
			badKeys:  []string{"has.Dot"},
			badChars: []string{`"."`},
		},
		{
			name:     "unicode_letters",
			goodKeys: []string{"世界"},
		},
		{
			name:     "unicode_numbers",
			goodKeys: []string{"٣٢١"},
		},
		{
			name:     "two_good",
			goodKeys: []string{"aardvark", "beagle"},
		},
		{
			name:     "one_good_one_bad",
			goodKeys: []string{"aardvark"},
			badKeys:  []string{"bad.key"},
			badChars: []string{`"."`},
		},
		{
			name:     "one_good_two_bad",
			goodKeys: []string{"aardvark"},
			badKeys:  []string{"bad.key", "also-bad"},
			badChars: []string{`"."`, `"-"`},
		},
		{
			name:     "repeated_bad_char",
			goodKeys: []string{"aardvark"},
			badKeys:  []string{"bad.key", "also.bad"},
			badChars: []string{`"."`, `"."`},
		},
		{
			name:     "repeated_bad_char_same_key",
			goodKeys: []string{"aardvark"},
			badKeys:  []string{"bad.key."},
			badChars: []string{`"."`},
		},
		{
			name:     "dont_escape",
			goodKeys: []string{"aardvark"},
			badKeys:  []string{"bad\\key"},
			badChars: []string{`"\"`},
		},
	}

	ci.Parallel(t)
	_, _, url := testServer(t, false, nil)

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			tc := tc       // capture test case
			ci.Parallel(t) // make the subtests parallel

			keys := append(tc.goodKeys, tc.badKeys...) // combine keys into a single slice
			for i, k := range keys {
				keys[i] = k + "=value" // Make each key into a k=v pair; value is not part of test
			}

			ui := cli.NewMockUi()
			cmd := &VarPutCommand{Meta: Meta{Ui: ui}}
			args := append([]string{"-address=" + url, "-force", "-out=json", "test/var"}, keys...)
			code := cmd.Run(args)
			errOut := ui.ErrorWriter.String()

			must.Eq(t, 0, code) // the command should always succeed

			badKeysLen := len(tc.badKeys)
			switch badKeysLen {
			case 0:
				must.Eq(t, "", errOut) // cases with no bad keys shouldn't put anything to stderr
				return
			case 1:
				must.StrContains(t, errOut, "1 warning:") // header should be singular
			default:
				must.StrContains(t, errOut, fmt.Sprintf("%d warnings:", badKeysLen)) // header should be plural
			}

			for _, k := range tc.badKeys {
				must.StrContains(t, errOut, k) // every bad key should appear in the warning output
			}

			if len(tc.badChars) > 0 {
				invalid := r.FindAllStringSubmatch(errOut, -1)
				for i, k := range tc.badChars {
					must.Eq(t, invalid[i][1], k) // every bad char should appear in the warning output
				}
			}

			for _, k := range tc.goodKeys {
				must.StrNotContains(t, errOut, k) // good keys should not be emitted
			}
		})
	}
}

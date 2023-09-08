// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/api/contexts"
	"github.com/mitchellh/cli"
	"github.com/mitchellh/colorstring"
	"github.com/mitchellh/mapstructure"
	"github.com/posener/complete"
)

type VarCommand struct {
	Meta
}

func (f *VarCommand) Help() string {
	helpText := `
Usage: nomad var <subcommand> [options] [args]

  This command groups subcommands for interacting with variables. Variables
  allow operators to provide credentials and otherwise sensitive material to
  Nomad jobs at runtime via the template block or directly through
  the Nomad API and CLI.

  Users can create new variables; list, inspect, and delete existing
  variables, and more. For a full guide on variables see:
  https://www.nomadproject.io/docs/concepts/variables

  Create a variable specification file:

      $ nomad var init

  Upsert a variable:

      $ nomad var put <path>

  Examine a variable:

      $ nomad var get <path>

  List existing variables:

      $ nomad var list <prefix>

  Purge a variable:

      $ nomad var purge <path>

  Please see the individual subcommand help for detailed usage information.
`

	return strings.TrimSpace(helpText)
}

func (f *VarCommand) Synopsis() string {
	return "Interact with variables"
}

func (f *VarCommand) Name() string { return "var" }

func (f *VarCommand) Run(args []string) int {
	return cli.RunResultHelp
}

// VariablePathPredictor returns a var predictor
func VariablePathPredictor(factory ApiClientFactory) complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) []string {
		client, err := factory()
		if err != nil {
			return nil
		}

		resp, _, err := client.Search().PrefixSearch(a.Last, contexts.Variables, nil)
		if err != nil {
			return []string{}
		}
		return resp.Matches[contexts.Variables]
	})
}

type VarUI interface {
	GetConcurrentUI() cli.ConcurrentUi
	Colorize() *colorstring.Colorize
}

// renderSVAsUiTable prints a variable as a table. It needs access to the
// command to get access to colorize and the UI itself. Commands that call it
// need to implement the VarUI interface.
func renderSVAsUiTable(sv *api.Variable, c VarUI) {
	meta := []string{
		fmt.Sprintf("Namespace|%s", sv.Namespace),
		fmt.Sprintf("Path|%s", sv.Path),
		fmt.Sprintf("Create Time|%v", formatUnixNanoTime(sv.ModifyTime)),
	}
	if sv.CreateTime != sv.ModifyTime {
		meta = append(meta, fmt.Sprintf("Modify Time|%v", time.Unix(0, sv.ModifyTime)))
	}
	meta = append(meta, fmt.Sprintf("Check Index|%v", sv.ModifyIndex))
	ui := c.GetConcurrentUI()
	ui.Output(formatKV(meta))
	ui.Output(c.Colorize().Color("\n[bold]Items[reset]"))
	items := make([]string, 0, len(sv.Items))

	keys := make([]string, 0, len(sv.Items))
	for k := range sv.Items {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		items = append(items, fmt.Sprintf("%s|%s", k, sv.Items[k]))
	}
	ui.Output(formatKV(items))
}

func renderAsHCL(sv *api.Variable) string {
	const tpl = `
namespace    = "{{.Namespace}}"
path         = "{{.Path}}"
create_index = {{.CreateIndex}}  # Set by server
modify_index = {{.ModifyIndex}}  # Set by server; consulted for check-and-set
create_time  = {{.CreateTime}}   # Set by server
modify_time  = {{.ModifyTime}}   # Set by server

items = {
{{- $PAD := 0 -}}{{- range $k,$v := .Items}}{{if gt (len $k) $PAD}}{{$PAD = (len $k)}}{{end}}{{end -}}
{{- $FMT := printf "  %%%vs = %%q\n" $PAD}}
{{range $k,$v := .Items}}{{printf $FMT $k $v}}{{ end -}}
}
`
	out, err := renderWithGoTemplate(sv, tpl)
	if err != nil {
		// Any errors in this should be caught as test panics.
		// If we ship with one, the worst case is that it panics a single
		// run of the CLI and only for output of variables in HCL.
		panic(err)
	}
	return out
}

func renderWithGoTemplate(sv *api.Variable, tpl string) (string, error) {
	//TODO: Enhance this to take a template as an @-aliased filename too
	t := template.Must(template.New("var").Parse(tpl))
	var out bytes.Buffer
	if err := t.Execute(&out, sv); err != nil {
		return "", err
	}

	result := out.String()
	return result, nil
}

// KVBuilder is a struct to build a key/value mapping based on a list
// of "k=v" pairs, where the value might come from stdin, a file, etc.
type KVBuilder struct {
	Stdin io.Reader

	result map[string]interface{}
	stdin  bool
}

// Map returns the built map.
func (b *KVBuilder) Map() map[string]interface{} {
	return b.result
}

// Add adds to the mapping with the given args.
func (b *KVBuilder) Add(args ...string) error {
	for _, a := range args {
		if err := b.add(a); err != nil {
			return fmt.Errorf("invalid key/value pair %q: %w", a, err)
		}
	}

	return nil
}

func (b *KVBuilder) add(raw string) error {
	// Regardless of validity, make sure we make our result
	if b.result == nil {
		b.result = make(map[string]interface{})
	}

	// Empty strings are fine, just ignored
	if raw == "" {
		return nil
	}

	// Split into key/value
	parts := strings.SplitN(raw, "=", 2)

	// If the arg is exactly "-", then we need to read from stdin
	// and merge the results into the resulting structure.
	if len(parts) == 1 {
		if raw == "-" {
			if b.Stdin == nil {
				return fmt.Errorf("stdin is not supported")
			}
			if b.stdin {
				return fmt.Errorf("stdin already consumed")
			}

			b.stdin = true
			return b.addReader(b.Stdin)
		}

		// If the arg begins with "@" then we need to read a file directly
		if raw[0] == '@' {
			f, err := os.Open(raw[1:])
			if err != nil {
				return err
			}
			defer f.Close()

			return b.addReader(f)
		}
	}

	if len(parts) != 2 {
		return fmt.Errorf("format must be key=value")
	}
	key, value := parts[0], parts[1]

	if len(value) > 0 {
		if value[0] == '@' {
			contents, err := os.ReadFile(value[1:])
			if err != nil {
				return fmt.Errorf("error reading file: %w", err)
			}

			value = string(contents)
		} else if value[0] == '\\' && value[1] == '@' {
			value = value[1:]
		} else if value == "-" {
			if b.Stdin == nil {
				return fmt.Errorf("stdin is not supported")
			}
			if b.stdin {
				return fmt.Errorf("stdin already consumed")
			}
			b.stdin = true

			var buf bytes.Buffer
			if _, err := io.Copy(&buf, b.Stdin); err != nil {
				return err
			}

			value = buf.String()
		}
	}

	// Repeated keys will be converted into a slice
	if existingValue, ok := b.result[key]; ok {
		var sliceValue []interface{}
		if err := mapstructure.WeakDecode(existingValue, &sliceValue); err != nil {
			return err
		}
		sliceValue = append(sliceValue, value)
		b.result[key] = sliceValue
		return nil
	}

	b.result[key] = value
	return nil
}

func (b *KVBuilder) addReader(r io.Reader) error {
	if r == nil {
		return fmt.Errorf("'io.Reader' being decoded is nil")
	}

	dec := json.NewDecoder(r)
	// While decoding JSON values, interpret the integer values as
	// `json.Number`s instead of `float64`.
	dec.UseNumber()

	return dec.Decode(&b.result)
}

// handleCASError provides consistent output for operations that result in a
// check-and-set error
func handleCASError(err error, c VarUI) (handled bool) {
	ui := c.GetConcurrentUI()
	var cErr api.ErrCASConflict
	if errors.As(err, &cErr) {
		lastUpdate := ""
		if cErr.Conflict.ModifyIndex > 0 {
			lastUpdate = fmt.Sprintf(
				tidyRawString(msgfmtCASConflictLastAccess),
				formatUnixNanoTime(cErr.Conflict.ModifyTime))
		}
		ui.Error(c.Colorize().Color("\n[bold][underline]Check-and-Set conflict[reset]\n"))
		ui.Warn(
			wrapAndPrepend(
				c.Colorize().Color(
					fmt.Sprintf(
						tidyRawString(msgfmtCASMismatch),
						cErr.CheckIndex,
						cErr.Conflict.ModifyIndex,
						lastUpdate),
				),
				80, "    ") + "\n",
		)
		handled = true
	}
	return
}

const (
	errMissingTemplate             = `A template must be supplied using '-template' when using go-template formatting`
	errUnexpectedTemplate          = `The '-template' flag is only valid when using 'go-template' formatting`
	errVariableNotFound            = `Variable not found`
	errNoMatchingVariables         = `No matching variables found`
	errInvalidInFormat             = `Invalid value for "-in"; valid values are [hcl, json]`
	errInvalidOutFormat            = `Invalid value for "-out"; valid values are [go-template, hcl, json, none, table]`
	errInvalidListOutFormat        = `Invalid value for "-out"; valid values are [go-template, json, table, terse]`
	errWildcardNamespaceNotAllowed = `The wildcard namespace ("*") is not valid for this command.`

	msgfmtCASMismatch = `
	Your provided check-index [green](%v)[yellow] does not match the
	server-side index [green](%v)[yellow].
	%s
	If you are sure you want to perform this operation, add the [green]-force[yellow] or
	[green]-check-index=%[2]v[yellow] flag before the positional arguments.`

	msgfmtCASConflictLastAccess = `
	The server-side item was last updated on [green]%s[yellow].
	`
)

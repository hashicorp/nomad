package command

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/api/contexts"
	"github.com/mitchellh/cli"
	"github.com/mitchellh/colorstring"
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
  Nomad jobs at runtime via the template stanza or directly through
  the Nomad API and CLI.

  Users can create new variables; list, inspect, and delete existing
  variables, and more. For a full guide on variables see:
  https://www.nomadproject.io/guides/vars.html

  Create a variable specification file:

      $ nomad var init

  Upsert a variable:

      $ nomad var put <path>

  Examine a variable:

      $ nomad var get <path>

  List existing variables:

      $ nomad var list <prefix>

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

// renderSVAsUiTable prints a secure variable as a table. It needs access to the
// command to get access to colorize and the UI itself. Commands that call it
// need to implement the VarUI interface.
func renderSVAsUiTable(sv *api.SecureVariable, c VarUI) {
	meta := []string{
		fmt.Sprintf("Namespace|%s", sv.Namespace),
		fmt.Sprintf("Path|%s", sv.Path),
		fmt.Sprintf("Create Time|%v", time.Unix(0, sv.ModifyTime)),
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

func renderAsHCL(sv *api.SecureVariable) string {
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
		// run of the CLI and only for output of secure variables in HCL.
		panic(err)
	}
	return out
}

func renderWithGoTemplate(sv *api.SecureVariable, tpl string) (string, error) {
	t := template.Must(template.New("var").Parse(tpl))
	var out bytes.Buffer
	if err := t.Execute(&out, sv); err != nil {
		return "", err
	}

	result := out.String()
	return result, nil
}

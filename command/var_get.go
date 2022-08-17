package command

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

type VarGetCommand struct {
	Meta
}

func (c *VarGetCommand) Help() string {
	helpText := `
Usage: nomad var get [options] <path>

  The 'var get' command is used to get the contents of an existing secure
  variable.

  If ACLs are enabled, this command requires a token with the 'var:read'
  capability.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Read Options:

  -format (table|json|hcl|go-template)
     Format to render the secure variable in. When using "go-template",
     provide the template content with the "-template" option. Defaults
     to "table"

  -template
     Template to render output with. Required when format is "go-template".

  -exit-code-not-found
     Exit code to use when the secure variable is not found. Defaults to
     1.
`
	return strings.TrimSpace(helpText)
}

func (c *VarGetCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-format":              complete.PredictSet("table", "hcl", "json", "go-template"),
			"-template":            complete.PredictAnything,
			"-exit-code-not-found": complete.PredictAnything,
		},
	)
}

func (c *VarGetCommand) AutocompleteArgs() complete.Predictor {
	return SecureVariablePathPredictor(c.Meta.Client)
}

func (c *VarGetCommand) Synopsis() string {
	return "Read a secure variable"
}

func (c *VarGetCommand) Name() string { return "var read" }

func (c *VarGetCommand) Run(args []string) int {
	var format, tmpl string
	var exitCodeNotFound int
	var out string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.StringVar(&format, "format", "table", "")
	flags.StringVar(&tmpl, "template", "", "")
	flags.IntVar(&exitCodeNotFound, "exit-code-not-found", 1, "")
	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got one argument
	args = flags.Args()
	if l := len(args); !(l == 1 || l == 2) {
		c.Ui.Error("This command takes one or two arguments:\n  <path>\n <path> <item key>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	if format == "go-template" && tmpl == "" {
		c.Ui.Error("A template must be supplied using '-template' when using go-template formatting")
		return 1
	}

	if format != "go-template" && tmpl != "" {
		c.Ui.Error("The '-template' flag is only valid when using 'go-template' formatting")
		return 1
	}

	path := args[0]

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	qo := &api.QueryOptions{
		Namespace: c.Meta.namespace,
	}

	sv, _, err := client.SecureVariables().Read(path, qo)
	if err != nil {
		if err.Error() == "secure variable not found" {
			c.Ui.Warn("Secure variable not found")
			return exitCodeNotFound
		}
		c.Ui.Error(fmt.Sprintf("Error retrieving secure variable: %s", err))
		return 1
	}

	switch format {
	case "json":
		out = sv.AsJSON()
	case "hcl":
		out, err = c.renderAsHCL(sv)
	case "go-template":
		out, err = c.renderGoTemplate(sv, tmpl)
	default:
		c.renderTable(sv)
		// the renderTable func writes directly to the ui
		// and doesn't error.
		return 0
	}
	if err != nil {
		c.Ui.Error(err.Error())
		return 1
	}
	c.Ui.Output(out)
	return 0
}

func (c *VarGetCommand) renderTable(sv *api.SecureVariable) {
	meta := []string{
		fmt.Sprintf("Namespace|%s", sv.Namespace),
		fmt.Sprintf("Path|%s", sv.Path),
		fmt.Sprintf("Create Time|%v", time.Unix(0, sv.ModifyTime)),
	}
	if sv.CreateTime != sv.ModifyTime {
		meta = append(meta, fmt.Sprintf("Modify Time|%v", time.Unix(0, sv.ModifyTime)))
	}
	meta = append(meta, fmt.Sprintf("Check Index|%v", sv.ModifyIndex))
	c.Ui.Output(formatKV(meta))
	c.Ui.Output(c.Colorize().Color("\n[bold]Items[reset]"))
	items := make([]string, 0, len(sv.Items))

	keys := make([]string, 0, len(sv.Items))
	for k := range sv.Items {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		items = append(items, fmt.Sprintf("%s|%s", k, sv.Items[k]))
	}
	c.Ui.Output(formatKV(items))
}

func (c *VarGetCommand) renderAsHCL(sv *api.SecureVariable) (string, error) {
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
	return c.renderGoTemplate(sv, tpl)
}

func (c *VarGetCommand) renderGoTemplate(sv *api.SecureVariable, tpl string) (string, error) {
	t := template.Must(template.New("var").Parse(tpl))
	var out bytes.Buffer
	if err := t.Execute(&out, sv); err != nil {
		return "", err
	}

	result := out.String()
	return result, nil
}

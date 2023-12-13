// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

const (
	msgWarnFilterPerformance = "Filter queries require a full scan of the data; use prefix searching where possible"
)

type VarListCommand struct {
	prefix string
	outFmt string
	tmpl   string
	Meta
}

func (c *VarListCommand) Help() string {
	helpText := `
Usage: nomad var list [options] <prefix>

  List is used to list available variables. Supplying an optional prefix,
  filters the list to variables having a path starting with the prefix.
  When using pagination, the next page token is provided in the JSON output
  or as a message to standard error to leave standard output for the listed
  variables from that page.

  If ACLs are enabled, this command will only return variables stored in
  namespaces and paths where the token has the 'variables:list' capability.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

List Options:

  -per-page
    How many results to show per page.

  -page-token
    Where to start pagination.

  -filter
    Specifies an expression used to filter query results. Queries using this
    option are less efficient than using the prefix parameter; therefore,
    the prefix parameter should be used whenever possible.

  -out (go-template | json | table | terse )
    Format to render created or updated variable. Defaults to "none" when
    stdout is a terminal and "json" when the output is redirected. The "terse"
	format outputs as little information as possible to uniquely identify a
	variable depending on whether or not the wildcard namespace was passed.

 -template
    Template to render output with. Required when format is "go-template",
    invalid for other formats.

`
	return strings.TrimSpace(helpText)
}

func (c *VarListCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-out":      complete.PredictSet("go-template", "json", "terse", "table"),
			"-template": complete.PredictAnything,
		},
	)
}

func (c *VarListCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *VarListCommand) Synopsis() string {
	return "List variable metadata"
}

func (c *VarListCommand) Name() string { return "var list" }
func (c *VarListCommand) Run(args []string) int {
	var perPage int
	var pageToken, filter, prefix string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.StringVar(&c.tmpl, "template", "", "")

	flags.IntVar(&perPage, "per-page", 0, "")
	flags.StringVar(&pageToken, "page-token", "", "")
	flags.StringVar(&filter, "filter", "", "")

	if fileInfo, _ := os.Stdout.Stat(); (fileInfo.Mode() & os.ModeCharDevice) != 0 {
		flags.StringVar(&c.outFmt, "out", "table", "")
	} else {
		flags.StringVar(&c.outFmt, "out", "json", "")
	}

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got no arguments
	args = flags.Args()
	if l := len(args); l > 1 {
		c.Ui.Error("This command takes flags and either no arguments or one: <prefix>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	if len(args) == 1 {
		prefix = args[0]
	}

	if err := c.validateOutputFlag(); err != nil {
		c.Ui.Error(err.Error())
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	if filter != "" {
		c.Ui.Warn(msgWarnFilterPerformance)
	}

	qo := &api.QueryOptions{
		Filter:    filter,
		PerPage:   int32(perPage),
		NextToken: pageToken,
		Params:    map[string]string{},
	}

	vars, qm, err := client.Variables().PrefixList(prefix, qo)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error retrieving vars: %s", err))
		return 1
	}

	switch c.outFmt {
	case "json":
		// obj and items enable us to rework the output before sending it
		// to the Format method for transformation into JSON.
		var obj, items interface{}
		obj = vars
		items = vars

		// If the response is paginated, we need to provide a means for the
		// caller to get to the pagination information. Wrapping the list
		// in a struct for the special case allows this extra data without
		// adding unnecessary structure in the non-paginated case.
		if perPage > 0 {
			obj = struct {
				Data      interface{}
				QueryMeta *api.QueryMeta
			}{
				items,
				qm,
			}
		}

		// By this point, the output is ready to be transformed to JSON via
		// the Format func.
		out, err := Format(true, "", obj)
		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}

		c.Ui.Output(out)

		// Since the JSON formatting deals with the pagination information
		// itself, exit the command here so that it doesn't double print.
		return 0

	case "terse":
		c.Ui.Output(
			formatList(
				dataToQuietStringSlice(vars, c.Meta.namespace)))

	case "go-template":
		out, err := Format(false, c.tmpl, vars)
		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}
		c.Ui.Output(out)

	default:
		c.Ui.Output(formatVarStubs(vars))
	}

	if qm.NextToken != "" {
		// This uses Ui.Warn to output the next page token to stderr
		// so that scripts consuming paths from stdout will not have
		// to special case the output.
		c.Ui.Warn(fmt.Sprintf("Next page token: %s", qm.NextToken))
	}

	return 0
}

func formatVarStubs(vars []*api.VariableMetadata) string {
	if len(vars) == 0 {
		return errNoMatchingVariables
	}

	// Sort the output by variable namespace, path
	sort.Slice(vars, func(i, j int) bool {
		if vars[i].Namespace == vars[j].Namespace {
			return vars[i].Path < vars[j].Path
		}
		return vars[i].Namespace < vars[j].Namespace
	})

	rows := make([]string, len(vars)+1)
	rows[0] = "Namespace|Path|Last Updated"
	for i, sv := range vars {
		rows[i+1] = fmt.Sprintf("%s|%s|%s",
			sv.Namespace,
			sv.Path,
			formatUnixNanoTime(sv.ModifyTime),
		)
	}
	return formatList(rows)
}

func dataToQuietStringSlice(vars []*api.VariableMetadata, ns string) []string {
	// If ns is the wildcard namespace, we have to provide namespace
	// as part of the quiet output, otherwise it can be a simple list
	// of paths.
	toPathStr := func(v *api.VariableMetadata) string {
		if ns == "*" {
			return fmt.Sprintf("%s|%s", v.Namespace, v.Path)
		}
		return v.Path
	}

	// Reduce the items slice to a string slice containing only the
	// variable paths.
	pList := make([]string, len(vars))
	for i, sv := range vars {
		pList[i] = toPathStr(sv)
	}

	return pList
}

func (c *VarListCommand) validateOutputFlag() error {
	if c.outFmt != "go-template" && c.tmpl != "" {
		return errors.New(errUnexpectedTemplate)
	}
	switch c.outFmt {
	case "json", "terse", "table":
		return nil
	case "go-template":
		if c.tmpl == "" {
			return errors.New(errMissingTemplate)
		}
		return nil
	default:
		return errors.New(errInvalidListOutFormat)
	}
}

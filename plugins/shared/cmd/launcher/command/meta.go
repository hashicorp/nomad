// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package command

import (
	"flag"
	"strings"

	"github.com/hashicorp/cli"
	hclog "github.com/hashicorp/go-hclog"
)

type Meta struct {
	Ui     cli.Ui
	logger hclog.Logger

	verbose bool
}

func NewMeta(ui cli.Ui, logger hclog.Logger) Meta {
	return Meta{
		Ui:     ui,
		logger: logger,
	}
}

func (m *Meta) FlagSet(n string) *flag.FlagSet {
	f := flag.NewFlagSet(n, flag.ContinueOnError)

	f.BoolVar(&m.verbose, "verbose", false, "Toggle verbose output")
	return f
}

// generalOptionsUsage return the help string for the global options
func generalOptionsUsage() string {
	helpText := `
  -verbose
    Enables verbose logging.
`
	return strings.TrimSpace(helpText)
}

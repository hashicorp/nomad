// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/posener/complete"
)

type ServiceDeleteCommand struct {
	Meta
}

func (s *ServiceDeleteCommand) Help() string {
	helpText := `
Usage: nomad service delete [options] <service_name> <service_id>

  Delete is used to deregister the specified service registration. It should be
  used with caution and can only remove a single registration, via the service
  name and service ID, at a time.

  When ACLs are enabled, this command requires a token with the 'submit-job'
  capability for the service registration namespace.

General Options:

  ` + generalOptionsUsage(usageOptsDefault)

	return strings.TrimSpace(helpText)
}

func (s *ServiceDeleteCommand) Name() string { return "service delete" }

func (s *ServiceDeleteCommand) Synopsis() string { return "Deregister a registered service" }

func (s *ServiceDeleteCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(s.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{})
}

func (s *ServiceDeleteCommand) Run(args []string) int {

	flags := s.Meta.FlagSet(s.Name(), FlagSetClient)
	if err := flags.Parse(args); err != nil {
		return 1
	}
	args = flags.Args()

	if len(args) != 2 {
		s.Ui.Error("This command takes two arguments: <service_name> and <service_id>")
		s.Ui.Error(commandErrorText(s))
		return 1
	}

	client, err := s.Meta.Client()
	if err != nil {
		s.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	if _, err := client.Services().Delete(args[0], args[1], nil); err != nil {
		s.Ui.Error(fmt.Sprintf("Error deleting service registration: %s", err))
		return 1
	}

	s.Ui.Output("Successfully deleted service registration")
	return 0
}

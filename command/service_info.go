package command

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
)

// Ensure ServiceInfoCommand satisfies the cli.Command interface.
var _ cli.Command = &ServiceInfoCommand{}

// ServiceInfoCommand implements cli.Command.
type ServiceInfoCommand struct {
	Meta
}

// Help satisfies the cli.Command Help function.
func (s *ServiceInfoCommand) Help() string {
	helpText := `
Usage: nomad service info [options] <service_name>

  Info is used to read the services registered to a single service name.

  When ACLs are enabled, this command requires a token with the 'read-job'
  capability for the service namespace.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Service Info Options:

  -verbose
    Display full information.

  -json
    Output the service in JSON format.

  -t
    Format and display the service using a Go template.
`
	return strings.TrimSpace(helpText)
}

// Synopsis satisfies the cli.Command Synopsis function.
func (s *ServiceInfoCommand) Synopsis() string {
	return "Display an individual Nomad service registration"
}

func (s *ServiceInfoCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(s.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-json":    complete.PredictNothing,
			"-t":       complete.PredictAnything,
			"-verbose": complete.PredictNothing,
		})
}

// Name returns the name of this command.
func (s *ServiceInfoCommand) Name() string { return "service info" }

// Run satisfies the cli.Command Run function.
func (s *ServiceInfoCommand) Run(args []string) int {
	var (
		json, verbose bool
		tmpl          string
	)

	flags := s.Meta.FlagSet(s.Name(), FlagSetClient)
	flags.Usage = func() { s.Ui.Output(s.Help()) }
	flags.BoolVar(&json, "json", false, "")
	flags.BoolVar(&verbose, "verbose", false, "")
	flags.StringVar(&tmpl, "t", "", "")
	if err := flags.Parse(args); err != nil {
		return 1
	}
	args = flags.Args()

	if len(args) != 1 {
		s.Ui.Error("This command takes one argument: <service_name>")
		s.Ui.Error(commandErrorText(s))
		return 1
	}

	client, err := s.Meta.Client()
	if err != nil {
		s.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	serviceInfo, _, err := client.Services().Get(args[0], nil)
	if err != nil {
		s.Ui.Error(fmt.Sprintf("Error listing service registrations: %s", err))
		return 1
	}

	if len(serviceInfo) == 0 {
		s.Ui.Output("No service registrations found")
		return 0
	}

	if json || len(tmpl) > 0 {
		out, err := Format(json, tmpl, serviceInfo)
		if err != nil {
			s.Ui.Error(err.Error())
			return 1
		}
		s.Ui.Output(out)
		return 0
	}

	// It is possible for multiple jobs to register a service with the same
	// name. In order to provide consistency, sort the output by job ID.
	sortedJobID := []string{}
	jobIDServices := make(map[string][]*api.ServiceRegistration)

	// Populate the objects, ensuring we do not add duplicate job IDs to the
	// array which will be sorted.
	for _, service := range serviceInfo {
		if _, ok := jobIDServices[service.JobID]; ok {
			jobIDServices[service.JobID] = append(jobIDServices[service.JobID], service)
		} else {
			jobIDServices[service.JobID] = []*api.ServiceRegistration{service}
			sortedJobID = append(sortedJobID, service.JobID)
		}
	}

	// Sort the jobIDs.
	sort.Strings(sortedJobID)

	if verbose {
		s.formatVerboseOutput(sortedJobID, jobIDServices)
	} else {
		s.formatOutput(sortedJobID, jobIDServices)
	}
	return 0
}

// formatOutput produces the non-verbose output of service registration info
// for a specific service by its name.
func (s *ServiceInfoCommand) formatOutput(jobIDs []string, jobServices map[string][]*api.ServiceRegistration) {

	// Create the output table header.
	outputTable := []string{"Job ID|Address|Tags|Node ID|Alloc ID"}

	// Populate the list.
	for _, jobID := range jobIDs {
		for _, service := range jobServices[jobID] {
			outputTable = append(outputTable, fmt.Sprintf(
				"%s|%s|[%s]|%s|%s",
				service.JobID,
				fmt.Sprintf("%s:%v", service.Address, service.Port),
				strings.Join(service.Tags, ","),
				limit(service.NodeID, shortId),
				limit(service.AllocID, shortId),
			))
		}
	}
	s.Ui.Output(formatList(outputTable))
}

// formatOutput produces the verbose output of service registration info for a
// specific service by its name.
func (s *ServiceInfoCommand) formatVerboseOutput(jobIDs []string, jobServices map[string][]*api.ServiceRegistration) {
	for _, jobID := range jobIDs {
		for _, service := range jobServices[jobID] {
			out := []string{
				fmt.Sprintf("ID|%s", service.ID),
				fmt.Sprintf("Service Name|%s", service.ServiceName),
				fmt.Sprintf("Namespace|%s", service.Namespace),
				fmt.Sprintf("Job ID|%s", service.JobID),
				fmt.Sprintf("Alloc ID|%s", service.AllocID),
				fmt.Sprintf("Node ID|%s", service.NodeID),
				fmt.Sprintf("Datacenter|%s", service.Datacenter),
				fmt.Sprintf("Address|%v", fmt.Sprintf("%s:%v", service.Address, service.Port)),
				fmt.Sprintf("Tags|[%s]\n", strings.Join(service.Tags, ",")),
			}
			s.Ui.Output(formatKV(out))
			s.Ui.Output("")
		}
	}
}

package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api/contexts"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
)

type StatusCommand struct {
	Meta
}

func (c *StatusCommand) Run(args []string) int {
	flags := c.Meta.FlagSet("status", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }

	if err := flags.Parse(args); err != nil {
		c.Ui.Error(fmt.Sprintf("Error parsing arguments: %q", err))
		return 1
	}

	// Store the original arguments so we can pass them to the routed command
	argsCopy := args

	// Check that we got exactly one evaluation ID
	args = flags.Args()

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %q", err))
		return 1
	}

	// If no identifier is provided, default to listing jobs
	if len(args) == 0 {
		cmd := &JobStatusCommand{Meta: c.Meta}
		return cmd.Run(argsCopy)
	}

	id := args[len(args)-1]

	// Query for the context associated with the id
	res, err := client.Search().PrefixSearch(id, contexts.All)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying search with id: %q", err))
		return 1
	}

	if res.Matches == nil {
		c.Ui.Error(fmt.Sprintf("No matches returned for query: %q", err))
		return 1
	}

	var match contexts.Context
	matchCount := 0
	for ctx, vers := range res.Matches {
		if len(vers) == 1 {
			match = ctx
			matchCount++
		}

		// Only a single result should return, as this is a match against a full id
		if matchCount > 1 || len(vers) > 1 {
			c.Ui.Error(fmt.Sprintf("Multiple matches found for id %s", id))
			return 1
		}
	}

	var cmd cli.Command
	switch match {
	case contexts.Evals:
		cmd = &EvalStatusCommand{Meta: c.Meta}
	case contexts.Nodes:
		cmd = &NodeStatusCommand{Meta: c.Meta}
	case contexts.Allocs:
		cmd = &AllocStatusCommand{Meta: c.Meta}
	case contexts.Jobs:
		cmd = &JobStatusCommand{Meta: c.Meta}
	default:
		c.Ui.Error(fmt.Sprintf("Expected a specific context for id : %s", id))
		return 1
	}

	return cmd.Run(argsCopy)
}

func (s *StatusCommand) Help() string {
	helpText := `Usage: nomad status <identifier>

	Display the status output for any given resource. The command will detect the type of resource being queried and display the appropriate status output.
	`
	return strings.TrimSpace(helpText)
}

func (s *StatusCommand) AutocompleteFlags() complete.Flags {
	return nil
}

func (s *StatusCommand) AutocompleteArgs() complete.Predictor {
	client, _ := s.Meta.Client()
	return complete.PredictFunc(func(a complete.Args) []string {
		if len(a.Completed) > 1 {
			return nil
		}

		resp, err := client.Search().PrefixSearch(a.Last, contexts.All)
		if err != nil {
			return []string{}
		}

		final := make([]string, 0)

		for _, matches := range resp.Matches {
			if len(matches) == 0 {
				continue
			}

			final = append(final, matches...)
		}

		return final
	})
}

func (c *StatusCommand) Synopsis() string {
	return "Display the status output for a resource"
}

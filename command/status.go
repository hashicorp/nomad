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

// Check that the last argument provided is not setting a flag
func lastArgIsFlag(args []string) bool {
	lastArg := args[len(args)-1]

	for _, flag := range flagOptions {
		arg := strings.Replace(lastArg, "-", "", 1) // strip leading '-' from flag

		if strings.HasPrefix(arg, flag) {
			return true
		}
	}
	return false
}

func (c *StatusCommand) Run(args []string) int {
	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	if len(args) == 0 || lastArgIsFlag(args) {
		cmd := &JobStatusCommand{Meta: c.Meta}
		return cmd.Run(args)
	}

	// Assume the last argument will be the id to search
	id := args[len(args)-1]

	// Try querying for the context associated with the id
	res, err := client.Search().PrefixSearch(id, contexts.All)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying search with id: %s", err))
		return 1
	}

	if res.Matches == nil {
		c.Ui.Error(fmt.Sprintf("No matches returned for query %s", err))
		return 1
	}

	var match contexts.Context
	matchCount := 0
	for ctx, vers := range res.Matches {
		if len(vers) == 1 {
			match = ctx
			matchCount++
		}

		// Only a single match should return, as this is a match against a full id
		if matchCount > 1 || len(vers) > 1 {
			c.Ui.Error(fmt.Sprintf("Multiple matches found for id %s", err))
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

	return cmd.Run(args)
}

func (s *StatusCommand) Help() string {
	helpText := `
Usage: nomad status <identifier>

	Display information about an existing resource. Job names, node ids,
	allocation ids, and evaluation ids are all valid identifiers.
	`
	return helpText
}

func (s *StatusCommand) AutocompleteFlags() complete.Flags {
	return nil
}

func (s *StatusCommand) AutocompleteArgs() complete.Predictor {
	client, _ := s.Meta.Client()
	return complete.PredictFunc(func(a complete.Args) []string {
		resp, err := client.Search().PrefixSearch(a.Last, contexts.All)
		if err != nil {
			return []string{}
		}
		return resp.Matches[contexts.All]
	})
}

func (c *StatusCommand) Synopsis() string {
	return "Display status information and metadata"
}

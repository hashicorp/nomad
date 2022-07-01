package command

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/api/contexts"
	"github.com/posener/complete"
)

type EvalDeleteCommand struct {
	Meta

	filter string
	yes    bool

	// deleteByArg is set when the command is deleting an evaluation that has
	// been passed as an argument. This avoids need for confirmation.
	deleteByArg bool

	// numDeleted tracks the total evaluations deleted in a single run of this
	// command. It provides a way to output this information to the user at the
	// command completion.
	numDeleted int

	// client is the lazy-loaded API client and is stored here, so we don't
	// need to pass it to multiple functions.
	client *api.Client
}

func (e *EvalDeleteCommand) Help() string {
	helpText := `
Usage: nomad eval delete [options] <evaluation>

  Delete an evaluation by ID. If the evaluation ID is omitted, this command
  will use the filter flag to identify and delete a set of evaluations. If ACLs
  are enabled, this command requires a management ACL token.

  This command should be used cautiously and only in outage situations where
  there is a large backlog of evaluations not being processed. During most
  normal and outage scenarios, Nomads reconciliation and state management will
  handle evaluations as needed.

  The eval broker is expected to be paused prior to running this command and
  un-paused after. This can be done using the following two commands:
    - nomad operator scheduler set-config -pause-eval-broker=true
    - nomad operator scheduler set-config -pause-eval-broker=false

General Options:

  ` + generalOptionsUsage(usageOptsNoNamespace) + `

Eval Delete Options:

  -filter
    Specifies an expression used to filter evaluations by for deletion. When
    using this flag, it is advisable to ensure the syntax is correct using the
    eval list command first.

  -yes
    Bypass the confirmation prompt if an evaluation ID was not provided.
`

	return strings.TrimSpace(helpText)
}

func (e *EvalDeleteCommand) Synopsis() string {
	return "Delete evaluations by ID or using a filter"
}

func (e *EvalDeleteCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(e.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-filter": complete.PredictAnything,
			"-yes":    complete.PredictNothing,
		})
}

func (e *EvalDeleteCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) []string {
		client, err := e.Meta.Client()
		if err != nil {
			return nil
		}

		resp, _, err := client.Search().PrefixSearch(a.Last, contexts.Evals, nil)
		if err != nil {
			return []string{}
		}
		return resp.Matches[contexts.Evals]
	})
}

func (e *EvalDeleteCommand) Name() string { return "eval delete" }

func (e *EvalDeleteCommand) Run(args []string) int {

	flags := e.Meta.FlagSet(e.Name(), FlagSetClient)
	flags.Usage = func() { e.Ui.Output(e.Help()) }
	flags.StringVar(&e.filter, "filter", "", "")
	flags.BoolVar(&e.yes, "yes", false, "")
	if err := flags.Parse(args); err != nil {
		return 1
	}

	args = flags.Args()

	if err := e.verifyArgsAndFlags(args); err != nil {
		e.Ui.Error(fmt.Sprintf("Error validating command args and flags: %v", err))
		return 1
	}

	// Get the HTTP client and store this for use across multiple functions.
	client, err := e.Meta.Client()
	if err != nil {
		e.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}
	e.client = client

	// Ensure the eval broker is paused. This check happens multiple times on
	// the leader, but this check means we can provide quick and actionable
	// feedback.
	schedulerConfig, _, err := e.client.Operator().SchedulerGetConfiguration(nil)
	if err != nil {
		e.Ui.Error(fmt.Sprintf("Error querying scheduler configuration: %s", err))
		return 1
	}

	if !schedulerConfig.SchedulerConfig.PauseEvalBroker {
		e.Ui.Error("Eval broker is not paused")
		e.Ui.Output(`To delete evaluations you must first pause the eval broker by running "nomad operator scheduler set-config -pause-eval-broker=true"`)
		e.Ui.Output(`After the deletion is complete, unpause the eval broker by running "nomad operator scheduler set-config -pause-eval-broker=false"`)
		return 1
	}

	// Track the eventual exit code as there are a number of factors that
	// influence this.
	var exitCode int

	// Call the correct function in order to handle the operator input
	// correctly.
	switch len(args) {
	case 1:
		e.deleteByArg = true
		exitCode, err = e.handleEvalArgDelete(args[0])
	default:

		// Track the next token, so we can iterate all pages that match the
		// passed filter.
		var nextToken string

		// It is possible the filter matches a large number of evaluations
		// which means we need to run a number of batch deletes. Perform
		// iteration here rather than recursion in later function, so we avoid
		// any potential issues with stack size limits.
		for {
			exitCode, nextToken, err = e.handleFlagFilterDelete(nextToken)

			// If there is another page of evaluations matching the filter,
			// iterate the loop and delete the next batch of evals. We pause
			// for a 500ms rather than just run as fast as the code and machine
			// possibly can. This means deleting 13million evals will take
			// roughly 13-15 mins, which seems reasonable. It is worth noting,
			// we do not expect operators to delete this many evals in a single
			// run and expect more careful filtering options to be used.
			if nextToken != "" {
				time.Sleep(500 * time.Millisecond)
				continue
			} else {
				break
			}
		}
	}

	// Do not exit if we got an error as it's possible this was on the
	// non-first iteration, and we have therefore deleted some evals.
	if err != nil {
		e.Ui.Error(fmt.Sprintf("Error deleting evaluations: %s", err))
	}

	// Depending on whether we deleted evaluations or not, output a message so
	// this is clear.
	if e.numDeleted > 0 {
		e.Ui.Output(fmt.Sprintf("Successfully deleted %v %s",
			e.numDeleted, correctGrammar("evaluation", e.numDeleted)))
	} else if err == nil {
		e.Ui.Output("No evaluations were deleted")
	}

	return exitCode
}

// verifyArgsAndFlags ensures the passed arguments and flags are valid for what
// this command accepts and can take action on.
func (e *EvalDeleteCommand) verifyArgsAndFlags(args []string) error {

	numArgs := len(args)

	// The command takes either an argument or filter, but not both.
	if (e.filter == "" && numArgs < 1) || (e.filter != "" && numArgs > 0) {
		return errors.New("evaluation ID or filter flag required")
	}

	// If an argument is supplied, we only accept a single eval ID.
	if numArgs > 1 {
		return fmt.Errorf("expected 1 argument, got %v", numArgs)
	}

	return nil
}

// handleEvalArgDelete handles deletion and evaluation which was passed via
// it's ID as a command argument. This is the simplest route to take and
// doesn't require filtering or batching.
func (e *EvalDeleteCommand) handleEvalArgDelete(evalID string) (int, error) {
	evalInfo, _, err := e.client.Evaluations().Info(evalID, nil)
	if err != nil {
		return 1, err
	}

	// Supplying an eval to delete by its ID will always skip verification, so
	// we don't need to understand the boolean response.
	code, _, err := e.batchDelete([]*api.Evaluation{evalInfo})
	return code, err
}

// handleFlagFilterDelete handles deletion of evaluations discovered using
// the filter. It is unknown how many will match the operator criteria so
// this function batches lookup and delete requests into sensible numbers.
func (e *EvalDeleteCommand) handleFlagFilterDelete(nt string) (int, string, error) {

	evalsToDelete, nextToken, err := e.batchLookupEvals(nt)
	if err != nil {
		return 1, "", err
	}

	numEvalsToDelete := len(evalsToDelete)

	// The filter flags are operator controlled, therefore ensure we
	// actually found some evals to delete. Otherwise, inform the operator
	// their flags are potentially incorrect.
	if numEvalsToDelete == 0 {
		if e.numDeleted > 0 {
			return 0, "", nil
		} else {
			return 1, "", errors.New("failed to find any evals that matched filter criteria")
		}
	}

	if code, actioned, err := e.batchDelete(evalsToDelete); err != nil {
		return code, "", err
	} else if !actioned {
		return code, "", nil
	}

	e.Ui.Info(fmt.Sprintf("Successfully deleted batch of %v %s",
		numEvalsToDelete, correctGrammar("evaluation", numEvalsToDelete)))

	return 0, nextToken, nil
}

// batchLookupEvals handles batched lookup of evaluations using the operator
// provided filter. The lookup is performed a maximum number of 3 times to
// ensure their size is limited and the number of evals to delete doesn't exceed
// the total allowable in a single call.
//
// The JSON serialized evaluation API object is 350-380B in size.
// 2426 * 380B (3.8e-4 MB) = 0.92MB. We may want to make this configurable
// in the future, but this is counteracted by the CLI logic which will loop
// until the user tells it to exit, or all evals matching the filter are
// deleted. 2426 * 3 falls below the maximum limit for eval IDs in a single
// delete request (set by MaxEvalIDsPerDeleteRequest).
func (e *EvalDeleteCommand) batchLookupEvals(nextToken string) ([]*api.Evaluation, string, error) {

	var evalsToDelete []*api.Evaluation
	currentNextToken := nextToken

	// Call List 3 times to accumulate the maximum number if eval IDs supported
	// in a single Delete request. See math above.
	for i := 0; i < 3; i++ {

		// Generate the query options using the passed next token and filter. The
		// per page value is less than the total number we can include in a single
		// delete request. This keeps the maximum size of the return object at a
		// reasonable size.
		opts := api.QueryOptions{
			Filter:    e.filter,
			PerPage:   2426,
			NextToken: currentNextToken,
		}

		evalList, meta, err := e.client.Evaluations().List(&opts)
		if err != nil {
			return nil, "", err
		}

		if len(evalList) > 0 {
			evalsToDelete = append(evalsToDelete, evalList...)
		}

		// Store the next token no matter if it is empty or populated.
		currentNextToken = meta.NextToken

		// If there is no next token, ensure we exit and avoid any new loops
		// which will result in duplicate IDs.
		if currentNextToken == "" {
			break
		}
	}

	return evalsToDelete, currentNextToken, nil
}

// batchDelete is responsible for deleting the passed evaluations and asking
// any confirmation questions along the way. It will ask whether the operator
// want to list the evals before deletion, and optionally ask for confirmation
// before deleting based on input criteria.
func (e *EvalDeleteCommand) batchDelete(evals []*api.Evaluation) (int, bool, error) {

	// Ask whether the operator wants to see the list of evaluations before
	// moving forward with deletion. This will only happen if filters are used
	// and the confirmation step is not bypassed.
	if !e.yes && !e.deleteByArg {
		_, listEvals := e.askQuestion(fmt.Sprintf(
			"Do you want to list evals (%v) before deletion? [y/N]",
			len(evals)), "")

		// List the evals for deletion is the user has requested this. It can
		// be useful when the list is small and targeted, but is maybe best
		// avoided when deleting large quantities of evals.
		if listEvals {
			e.Ui.Output("")
			outputEvalList(e.Ui, evals, shortId)
			e.Ui.Output("")
		}
	}

	// Generate our list of eval IDs which is required for the API request.
	ids := make([]string, len(evals))

	for i, eval := range evals {
		ids[i] = eval.ID
	}

	// If the user did not wish to bypass the confirmation step, ask this now
	// and handle the response.
	if !e.yes && !e.deleteByArg {
		code, deleteEvals := e.askQuestion(fmt.Sprintf(
			"Are you sure you want to delete %v evals? [y/N]",
			len(evals)), "Cancelling eval deletion")
		e.Ui.Output("")

		if !deleteEvals {
			return code, deleteEvals, nil
		}
	}

	_, err := e.client.Evaluations().Delete(ids, nil)
	if err != nil {
		return 1, false, err
	}

	// Calculate how many total evaluations we have deleted, so we can output
	// this at the end of the process.
	curDeleted := e.numDeleted
	e.numDeleted = curDeleted + len(ids)

	return 0, true, nil
}

// askQuestion allows the command to ask the operator a question requiring a
// y/n response. The optional noResp is used when the operator responds no to
// a question.
func (e *EvalDeleteCommand) askQuestion(question, noResp string) (int, bool) {

	answer, err := e.Ui.Ask(question)
	if err != nil {
		e.Ui.Error(fmt.Sprintf("Failed to parse answer: %v", err))
		return 1, false
	}

	if answer == "" || strings.ToLower(answer)[0] == 'n' {
		if noResp != "" {
			e.Ui.Output(noResp)
		}
		return 0, false
	} else if strings.ToLower(answer)[0] == 'y' && len(answer) > 1 {
		e.Ui.Output("For confirmation, an exact ‘y’ is required.")
		return 0, false
	} else if answer != "y" {
		e.Ui.Output("No confirmation detected. For confirmation, an exact 'y' is required.")
		return 1, false
	}
	return 0, true
}

func correctGrammar(word string, num int) string {
	if num > 1 {
		return word + "s"
	}
	return word
}

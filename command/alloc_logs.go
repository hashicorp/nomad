package command

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/api/contexts"
	"github.com/posener/complete"
)

type AllocLogsCommand struct {
	Meta
}

func (l *AllocLogsCommand) Help() string {
	helpText := `
Usage: nomad alloc logs [options] <allocation> <task>
Alias: nomad logs

  Streams the stdout/stderr of the given allocation and task.

  When ACLs are enabled, this command requires a token with the 'read-logs',
  'read-job', and 'list-jobs' capabilities for the allocation's namespace.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Logs Specific Options:

  -stderr
    Display stderr logs.

  -verbose
    Show full information.

  -task <task-name>
    Sets the task to view the logs. If task name is given with both an argument 
	and the '-task' option, preference is given to the '-task' option.

  -job <job-id>
    Use a random allocation from the specified job ID.

  -f
    Causes the output to not stop when the end of the logs are reached, but
    rather to wait for additional output.

  -tail
    Show the logs contents with offsets relative to the end of the logs. If no
    offset is given, -n is defaulted to 10.

  -n
    Sets the tail location in best-efforted number of lines relative to the end
    of the logs.

  -c
    Sets the tail location in number of bytes relative to the end of the logs.

  Note that the -no-color option applies to Nomad's own output. If the task's
  logs include terminal escape sequences for color codes, Nomad will not
  remove them.
`

	return strings.TrimSpace(helpText)
}

func (l *AllocLogsCommand) Synopsis() string {
	return "Streams the logs of a task."
}

func (l *AllocLogsCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(l.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-stderr":  complete.PredictNothing,
			"-verbose": complete.PredictNothing,
			"-task":    complete.PredictAnything,
			"-job":     complete.PredictAnything,
			"-f":       complete.PredictNothing,
			"-tail":    complete.PredictAnything,
			"-n":       complete.PredictAnything,
			"-c":       complete.PredictAnything,
		})
}

func (l *AllocLogsCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) []string {
		client, err := l.Meta.Client()
		if err != nil {
			return nil
		}

		resp, _, err := client.Search().PrefixSearch(a.Last, contexts.Allocs, nil)
		if err != nil {
			return []string{}
		}
		return resp.Matches[contexts.Allocs]
	})
}

func (l *AllocLogsCommand) Name() string { return "alloc logs" }

func (l *AllocLogsCommand) Run(args []string) int {
	var verbose, job, tail, stderr, follow bool
	var numLines, numBytes int64
	var task string

	flags := l.Meta.FlagSet(l.Name(), FlagSetClient)
	flags.Usage = func() { l.Ui.Output(l.Help()) }
	flags.BoolVar(&verbose, "verbose", false, "")
	flags.BoolVar(&job, "job", false, "")
	flags.BoolVar(&tail, "tail", false, "")
	flags.BoolVar(&follow, "f", false, "")
	flags.BoolVar(&stderr, "stderr", false, "")
	flags.Int64Var(&numLines, "n", -1, "")
	flags.Int64Var(&numBytes, "c", -1, "")
	flags.StringVar(&task, "task", "", "")

	if err := flags.Parse(args); err != nil {
		return 1
	}
	args = flags.Args()

	if numArgs := len(args); numArgs < 1 {
		if job {
			l.Ui.Error("A job ID is required")
		} else {
			l.Ui.Error("An allocation ID is required")
		}

		l.Ui.Error(commandErrorText(l))
		return 1
	} else if numArgs > 2 {
		l.Ui.Error("This command takes one or two arguments")
		l.Ui.Error(commandErrorText(l))
		return 1
	}

	client, err := l.Meta.Client()
	if err != nil {
		l.Ui.Error(fmt.Sprintf("Error initializing client: %v", err))
		return 1
	}

	// If -job is specified, use random allocation, otherwise use provided allocation
	allocID := args[0]
	if job {
		allocID, err = getRandomJobAllocID(client, args[0])
		if err != nil {
			l.Ui.Error(fmt.Sprintf("Error fetching allocations: %v", err))
			return 1
		}
	}

	// Truncate the id unless full length is requested
	length := shortId
	if verbose {
		length = fullId
	}
	// Query the allocation info
	if len(allocID) == 1 {
		l.Ui.Error("Alloc ID must contain at least two characters.")
		return 1
	}

	allocID = sanitizeUUIDPrefix(allocID)
	allocs, _, err := client.Allocations().PrefixList(allocID)
	if err != nil {
		l.Ui.Error(fmt.Sprintf("Error querying allocation: %v", err))
		return 1
	}
	if len(allocs) == 0 {
		l.Ui.Error(fmt.Sprintf("No allocation(s) with prefix or id %q found", allocID))
		return 1
	}
	if len(allocs) > 1 {
		// Format the allocs
		out := formatAllocListStubs(allocs, verbose, length)
		l.Ui.Error(fmt.Sprintf("Prefix matched multiple allocations\n\n%s", out))
		return 1
	}
	// Prefix lookup matched a single allocation
	q := &api.QueryOptions{Namespace: allocs[0].Namespace}
	alloc, _, err := client.Allocations().Info(allocs[0].ID, q)
	if err != nil {
		l.Ui.Error(fmt.Sprintf("Error querying allocation: %s", err))
		return 1
	}

	// If -task isn't provided fallback to reading the task name
	// from args.
	if task != "" {
		err = validateTaskExistsInAllocation(task, alloc)
	} else {
		if len(args) >= 2 {
			task = args[1]
			if task == "" {
				l.Ui.Error("Task name required")
				return 1
			}
		} else {
			task, err = lookupAllocTask(alloc)
		}
	}
	if err != nil {
		l.Ui.Error(fmt.Sprintf("Failed to validate task: %s", err))
		return 1
	}

	logType := "stdout"
	if stderr {
		logType = "stderr"
	}

	// We have a file, output it.
	var r io.ReadCloser
	var readErr error
	if !tail {
		r, readErr = l.followFile(client, alloc, follow, task, logType, api.OriginStart, 0)
		if readErr != nil {
			readErr = fmt.Errorf("Error reading file: %v", readErr)
		}
	} else {
		// Parse the offset
		var offset int64 = defaultTailLines * bytesToLines

		if nLines, nBytes := numLines != -1, numBytes != -1; nLines && nBytes {
			l.Ui.Error("Both -n and -c set")
			return 1
		} else if nLines {
			offset = numLines * bytesToLines
		} else if nBytes {
			offset = numBytes
		} else {
			numLines = defaultTailLines
		}

		r, readErr = l.followFile(client, alloc, follow, task, logType, api.OriginEnd, offset)

		// If numLines is set, wrap the reader
		if numLines != -1 {
			r = NewLineLimitReader(r, int(numLines), int(numLines*bytesToLines), 1*time.Second)
		}

		if readErr != nil {
			readErr = fmt.Errorf("Error tailing file: %v", readErr)
		}
	}

	if readErr != nil {
		l.Ui.Error(readErr.Error())
		return 1
	}

	defer r.Close()
	_, err = io.Copy(os.Stdout, r)
	if err != nil {
		l.Ui.Error(fmt.Sprintf("error following logs: %s", err))
		return 1
	}

	return 0
}

// followFile outputs the contents of the file to stdout relative to the end of
// the file.
func (l *AllocLogsCommand) followFile(client *api.Client, alloc *api.Allocation,
	follow bool, task, logType, origin string, offset int64) (io.ReadCloser, error) {

	cancel := make(chan struct{})
	frames, errCh := client.AllocFS().Logs(alloc, follow, task, logType, origin, offset, cancel, nil)
	select {
	case err := <-errCh:
		return nil, err
	default:
	}
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)

	// Create a reader
	var r io.ReadCloser
	frameReader := api.NewFrameReader(frames, errCh, cancel)
	frameReader.SetUnblockTime(500 * time.Millisecond)
	r = frameReader

	go func() {
		<-signalCh

		// End the streaming
		r.Close()
	}()

	return r, nil
}

func lookupAllocTask(alloc *api.Allocation) (string, error) {
	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
	if tg == nil {
		return "", fmt.Errorf("Could not find allocation task group: %s", alloc.TaskGroup)
	}

	if len(tg.Tasks) == 1 {
		return tg.Tasks[0].Name, nil
	}

	var errStr strings.Builder
	fmt.Fprintf(&errStr, "Allocation %q is running the following tasks:\n", limit(alloc.ID, shortId))
	for _, t := range tg.Tasks {
		fmt.Fprintf(&errStr, "  * %s\n", t.Name)
	}
	fmt.Fprintf(&errStr, "\nPlease specify the task.")
	return "", errors.New(errStr.String())
}

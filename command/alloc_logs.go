// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

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
	"github.com/hashicorp/nomad/command/ui"
	"github.com/posener/complete"
)

type AllocLogsCommand struct {
	Meta

	// The fields below represent the commands flags.
	verbose, job, tail, stderr, stdout, follow bool
	numLines                                   int64
	numBytes                                   int64
	task                                       string
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

  -stdout
    Display stdout logs. This is used as the default value in all commands
    except when using the "-f" flag where both stdout and stderr are used as
    default.

  -stderr
    Display stderr logs.

  -verbose
    Show full information.

  -task <task-name>
    Sets the task to view the logs. If task name is given with both an argument
	and the '-task' option, preference is given to the '-task' option.

  -job <job-id>
    Use a random allocation from the specified job ID or prefix.

  -f
    Causes the output to not stop when the end of the logs are reached, but
    rather to wait for additional output. When supplied with no other flags
    except optionally "-job" and "-task", both stdout and stderr logs will be
    followed.

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
			"-stdout":  complete.PredictNothing,
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

	flags := l.Meta.FlagSet(l.Name(), FlagSetClient)
	flags.Usage = func() { l.Ui.Output(l.Help()) }
	flags.BoolVar(&l.verbose, "verbose", false, "")
	flags.BoolVar(&l.job, "job", false, "")
	flags.BoolVar(&l.tail, "tail", false, "")
	flags.BoolVar(&l.follow, "f", false, "")
	flags.BoolVar(&l.stderr, "stderr", false, "")
	flags.BoolVar(&l.stdout, "stdout", false, "")
	flags.Int64Var(&l.numLines, "n", -1, "")
	flags.Int64Var(&l.numBytes, "c", -1, "")
	flags.StringVar(&l.task, "task", "", "")

	if err := flags.Parse(args); err != nil {
		return 1
	}
	args = flags.Args()

	if numArgs := len(args); numArgs < 1 {
		if l.job {
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
	if l.job {
		jobID, ns, err := l.JobIDByPrefix(client, args[0], nil)
		if err != nil {
			l.Ui.Error(err.Error())
			return 1
		}

		allocID, err = getRandomJobAllocID(client, jobID, ns)
		if err != nil {
			l.Ui.Error(fmt.Sprintf("Error fetching allocations: %v", err))
			return 1
		}
	}

	// Truncate the id unless full length is requested
	length := shortId
	if l.verbose {
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
		out := formatAllocListStubs(allocs, l.verbose, length)
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
	if l.task != "" {
		err = validateTaskExistsInAllocation(l.task, alloc)
	} else {
		if len(args) >= 2 {
			l.task = args[1]
			if l.task == "" {
				l.Ui.Error("Task name required")
				return 1
			}
		} else {
			l.task, err = lookupAllocTask(alloc)
		}
	}
	if err != nil {
		l.Ui.Error(fmt.Sprintf("Failed to validate task: %s", err))
		return 1
	}

	// In order to run the mixed log output, we can only follow the files from
	// their current positions. There is no way to interleave previous log
	// lines as there is no timestamp references.
	if l.follow && !(l.stderr || l.stdout || l.tail || l.numLines > 0 || l.numBytes > 0) {
		if err := l.tailMultipleFiles(client, alloc); err != nil {
			l.Ui.Error(fmt.Sprintf("Failed to tail stdout and stderr files: %v", err))
			return 1
		}
	} else {

		// If we are not strictly following the two files, we cannot support
		// specifying both are targets.
		if l.stderr && l.stdout {
			l.Ui.Error("Unable to support both stdout and stderr")
			return 1
		}

		logType := api.FSLogNameStdout
		if l.stderr {
			logType = api.FSLogNameStderr
		}
		if err := l.handleSingleFile(client, alloc, logType); err != nil {
			l.Ui.Error(fmt.Sprintf("Failed to read %s file: %v", logType, err))
			return 1
		}
	}

	return 0
}

func (l *AllocLogsCommand) handleSingleFile(client *api.Client, alloc *api.Allocation, logType string) error {
	// We have a file, output it.
	var r io.ReadCloser
	var readErr error
	if !l.tail {
		r, readErr = l.followFile(client, alloc, logType, api.OriginStart, 0)
		if readErr != nil {
			return fmt.Errorf("error reading file: %v", readErr)
		}
	} else {
		// Parse the offset
		var offset = defaultTailLines * bytesToLines

		if nLines, nBytes := l.numLines != -1, l.numBytes != -1; nLines && nBytes {
			return errors.New("both -n and -c set")
		} else if nLines {
			offset = l.numLines * bytesToLines
		} else if nBytes {
			offset = l.numBytes
		} else {
			l.numLines = defaultTailLines
		}

		r, readErr = l.followFile(client, alloc, logType, api.OriginEnd, offset)

		// If numLines is set, wrap the reader
		if l.numLines != -1 {
			r = NewLineLimitReader(r, int(l.numLines), int(l.numLines*bytesToLines), 1*time.Second)
		}

		if readErr != nil {
			return fmt.Errorf("error tailing file: %v", readErr)
		}
	}

	defer r.Close()
	if _, err := io.Copy(os.Stdout, r); err != nil {
		return fmt.Errorf("error following logs: %s", err)
	}

	return nil
}

// followFile outputs the contents of the file to stdout relative to the end of
// the file.
func (l *AllocLogsCommand) followFile(client *api.Client, alloc *api.Allocation,
	logType, origin string, offset int64) (io.ReadCloser, error) {

	cancel := make(chan struct{})
	frames, errCh := client.AllocFS().Logs(alloc, l.follow, l.task, logType, origin, offset, cancel, nil)

	// Setting up the logs stream can fail, therefore we need to check the
	// error channel before continuing further.
	select {
	case err := <-errCh:
		return nil, err
	default:
	}

	// Create a reader but don't initially cast it to an io.ReadCloser so that
	// we can set the unblock time.
	var r io.ReadCloser
	frameReader := api.NewFrameReader(frames, errCh, cancel)
	frameReader.SetUnblockTime(500 * time.Millisecond)
	r = frameReader

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)

	// This go routine blocks until the command receives an interrupt or
	// terminate signal, at which point we close the ReadCloser.
	go func() {
		<-signalCh
		_ = r.Close()
	}()

	return r, nil
}

// tailMultipleFiles will follow both stdout and stderr log files of the passed
// allocation. Each stream will be output to the users console via stout and
// stderr until the user cancels it.
func (l *AllocLogsCommand) tailMultipleFiles(client *api.Client, alloc *api.Allocation) error {

	// Use a single cancel channel for both log streams, so we only have to
	// close one.
	cancel := make(chan struct{})

	// Ensure the channel is closed in order to notify listeners whenever we
	// exit.
	defer close(cancel)

	stdoutFrames, stdoutErrCh := client.AllocFS().Logs(
		alloc, true, l.task, api.FSLogNameStdout, api.OriginEnd, 1, cancel, nil)

	// Setting up the logs stream can fail, therefore we need to check the
	// error channel before continuing further.
	select {
	case err := <-stdoutErrCh:
		return fmt.Errorf("failed to setup stdout log tailing: %v", err)
	default:
	}

	stderrFrames, stderrErrCh := client.AllocFS().Logs(
		alloc, true, l.task, api.FSLogNameStderr, api.OriginEnd, 1, cancel, nil)

	// Setting up the logs stream can fail, therefore we need to check the
	// error channel before continuing further.
	select {
	case err := <-stderrErrCh:
		return fmt.Errorf("failed to setup stderr log tailing: %v", err)
	default:
	}

	// Trap user signals, so we know when to exit and cancel the log streams
	// running in the background.
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)

	// Generate our logging UI that doesn't add any additional formatting to
	// output strings.
	logUI, err := ui.NewLogUI(l.Ui)
	if err != nil {
		return err
	}

	// Enter the main loop where we listen for log frames, errors, and a cancel
	// signal. Any error at this point will result in the stream being ended,
	// therefore should result in this command exiting. Otherwise, we would
	// just be printing a single stream, which might be hard to notice for the
	// user.
	for {
		select {
		case <-signalCh:
			return nil
		case stdoutErr := <-stdoutErrCh:
			return fmt.Errorf("received an error from stdout log stream: %v", stdoutErr)
		case stdoutFrame := <-stdoutFrames:
			if stdoutFrame != nil {
				logUI.Output(string(stdoutFrame.Data))
			}
		case stderrErr := <-stderrErrCh:
			return fmt.Errorf("received an error from stderr log stream: %v", stderrErr)
		case stderrFrame := <-stderrFrames:
			if stderrFrame != nil {
				logUI.Warn(string(stderrFrame.Data))
			}
		}
	}
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

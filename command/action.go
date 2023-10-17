// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/api/contexts"
	"github.com/hashicorp/nomad/helper/escapingio"
	"github.com/posener/complete"
)

type ActionCommand struct {
	Meta

	Stdin  io.Reader
	Stdout io.WriteCloser
	Stderr io.WriteCloser
}

// TODO: Verify that I don't need to add Group here afterall
func (l *ActionCommand) Help() string {
	helpText := `
Usage: nomad action [options] <action>

  Perform a predefined command inside the environment of the given allocation
  and task.

  When ACLs are enabled, this command requires a token with the 'alloc-exec',
  'read-job', and 'list-jobs' capabilities for a task's namespace. If
  the task driver does not have file system isolation (as with 'raw_exec'),
  this command requires the 'alloc-node-exec', 'read-job', and 'list-jobs'
  capabilities for the task's namespace.

General Options:

  ` + generalOptionsUsage(usageOptsNoNamespace) + `

Action Specific Options:

  -task <task-name>
	  Specifies the task in which the Action is defined

	-job <job-id>
		Specifies the job in which the Action is defined

  -allocation <allocation-id>
    Specifies the allocation in which the Action is defined
  `
	return strings.TrimSpace(helpText)
}

func (l *ActionCommand) Synopsis() string {
	return "Run a pre-defined action from a Nomad task"
}

func (l *ActionCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(l.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-task":       complete.PredictAnything,
			"-job":        complete.PredictAnything,
			"-allocation": complete.PredictAnything,
		})
}

func (l *ActionCommand) AutocompleteArgs() complete.Predictor {
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

func (l *ActionCommand) Name() string { return "action" }

func (l *ActionCommand) Run(args []string) int {

	// Log running
	l.Ui.Output(fmt.Sprintf("Running command: %s", l.Name()))

	// Log to server

	var stdinOpt, ttyOpt bool
	var task, allocation, job, escapeChar string

	flags := l.Meta.FlagSet(l.Name(), FlagSetClient)
	flags.Usage = func() { l.Ui.Output(l.Help()) }
	flags.StringVar(&task, "task", "", "")
	flags.StringVar(&allocation, "allocation", "", "")
	flags.StringVar(&job, "job", "", "")
	// TODO: add namespace flag
	l.Ui.Output(fmt.Sprintf("Parsed Flags: Allocation=%s, Task=%s, Job=%s", allocation, task, job))

	// Log out flags back to me
	l.Ui.Output(fmt.Sprintf("Flags: %s", flags))

	// flags.BoolVar(&stdinOpt, "i", true, "")
	// flags.BoolVar(&ttyOpt, "t", isTty(), "")
	// flags.StringVar(&escapeChar, "e", "~", "")

	if err := flags.Parse(args); err != nil {
		// log err
		l.Ui.Output(fmt.Sprintf("Error parsing flags: %s", err))
		return 1
	}

	args = flags.Args()

	if len(args) < 1 {
		l.Ui.Error(fmt.Sprintf("An action name is required"))
		return 1
	}

	if allocation == "" {
		l.Ui.Error("An allocation ID is required")
		return 1
	}

	if job == "" {
		l.Ui.Error("A job ID is required")
		return 1
	}

	if task == "" {
		l.Ui.Error("A task name is required")
		return 1
	}

	if ttyOpt && !stdinOpt {
		l.Ui.Error("-i must be enabled if running with tty")
		return 1
	}

	if escapeChar == "none" {
		escapeChar = ""
	}

	if len(escapeChar) > 1 {
		l.Ui.Error("-e requires 'none' or a single character")
		return 1
	}

	client, err := l.Meta.Client()
	if err != nil {
		l.Ui.Error(fmt.Sprintf("Error initializing client: %v", err))
		return 1
	}

	var allocStub *api.AllocationListStub
	if job != "" {
		// change from bool condition to string
		// jobID, ns, err := l.JobIDByPrefix(client, args[0], nil)
		// if err != nil {
		// 	l.Ui.Error(err.Error())
		// 	return 1
		// }

		ns := "default" // TODO: TEMP

		l.Ui.Output(fmt.Sprintf("job passed: %s", job))
		l.Ui.Output(fmt.Sprintf("Namespace: %s", ns))
		l.Ui.Output(fmt.Sprintf("action name: %s", args[0]))

		allocStub, err = getRandomJobAlloc(client, job, ns)
		if err != nil {
			l.Ui.Error(fmt.Sprintf("Error fetching allocations: %v", err))
			return 1
		}
		l.Ui.Output(fmt.Sprintf("allocStub: %s", allocStub))
	} else {
		// I think irrelevant?
		// allocID := args[0]
		// allocs, _, err := client.Allocations().PrefixList(sanitizeUUIDPrefix(allocID))
		// if err != nil {
		// 	l.Ui.Error(fmt.Sprintf("Error querying allocation: %v", err))
		// 	return 1
		// }

		// if len(allocs) == 0 {
		// 	l.Ui.Error(fmt.Sprintf("No allocation(s) with prefix or id %q found", allocID))
		// 	return 1
		// }

		// if len(allocs) > 1 {
		// 	out := formatAllocListStubs(allocs, false, shortId)
		// 	l.Ui.Error(fmt.Sprintf("Prefix matched multiple allocations\n\n%s", out))
		// 	return 1
		// }

		// allocStub = allocs[0]
	}

	q := &api.QueryOptions{Namespace: allocStub.Namespace}
	alloc, _, err := client.Allocations().Info(allocStub.ID, q)
	if err != nil {
		l.Ui.Error(fmt.Sprintf("Error querying allocation: %s", err))
		return 1
	}

	if task != "" {
		err = validateTaskExistsInAllocation(task, alloc)
	} else {
		task, err = lookupAllocTask(alloc)
	}
	if err != nil {
		l.Ui.Error(err.Error())
		return 1
	}
	l.Ui.Output(fmt.Sprintf("Task set: %s", task))

	if !stdinOpt {
		l.Stdin = bytes.NewReader(nil)
	}

	if l.Stdin == nil {
		l.Stdin = os.Stdin
	}

	if l.Stdout == nil {
		l.Stdout = os.Stdout
	}

	if l.Stderr == nil {
		l.Stderr = os.Stderr
	}

	action := args[0]
	// Convert action to command here.
	// TODO: Noting that this is also being (redundantly) done in client/alloc_endpoint.go's execImpl.
	// I could pass the action name all the way down, but that would be maybe polluting the param space of
	// api/allocations.go, api/allocations_exec.go, etc., since alloc exec normally would never use an Action
	// except by via this command.
	// TODO: update: I think I'll try that anyway.

	code, err := l.execImpl(client, alloc, task, job, action, ttyOpt, escapeChar, l.Stdin, l.Stdout, l.Stderr)
	if err != nil {
		l.Ui.Error(fmt.Sprintf("failed to exec into task: %v", err))
		return 1
	}

	return code
}

// execImpl invokes the Alloc Exec api call, it also prepares and restores terminal states as necessary.
func (l *ActionCommand) execImpl(client *api.Client, alloc *api.Allocation, task string, job string, action string, tty bool,
	escapeChar string, stdin io.Reader, stdout, stderr io.WriteCloser) (int, error) {

	l.Ui.Output(fmt.Sprintf("Impl command: %s", l.Name()))

	sizeCh := make(chan api.TerminalSize, 1)

	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()

	// When tty, ensures we capture all user input and monitor terminal resizes.
	if tty {
		if stdin == nil {
			return -1, fmt.Errorf("stdin is null")
		}

		inCleanup, err := setRawTerminal(stdin)
		if err != nil {
			return -1, err
		}
		defer inCleanup()

		outCleanup, err := setRawTerminalOutput(stdout)
		if err != nil {
			return -1, err
		}
		defer outCleanup()

		sizeCleanup, err := watchTerminalSize(stdout, sizeCh)
		if err != nil {
			return -1, err
		}
		defer sizeCleanup()

		if escapeChar != "" {
			stdin = escapingio.NewReader(stdin, escapeChar[0], func(c byte) bool {
				switch c {
				case '.':
					// need to restore tty state so error reporting here
					// gets emitted at beginning of line
					outCleanup()
					inCleanup()

					stderr.Write([]byte("\nConnection closed\n"))
					cancelFn()
					return true
				default:
					return false
				}
			})
		}
	}

	// TODO: establish command here from action.

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		for range signalCh {
			cancelFn()
		}
	}()

	// TODO: is make([]string, 0) the right thing to pass an empty command here?
	return client.Allocations().Exec(ctx,
		alloc, task, tty, make([]string, 0), action, stdin, stdout, stderr, sizeCh, nil)
}

// isTty returns true if both stdin and stdout are a TTY
// TODO: why does this matter?
// func isTty() bool {
// 	_, isStdinTerminal := term.GetFdInfo(os.Stdin)
// 	_, isStdoutTerminal := term.GetFdInfo(os.Stdout)
// 	return isStdinTerminal && isStdoutTerminal
// }

// TODO: do we want this? Commands should be one-shot, but still ctrl-c-able?
// setRawTerminal sets the stream terminal in raw mode, so process captures
// Ctrl+C and other commands to forward to remote process.
// It returns a cleanup function that restores terminal to original mode.
// func setRawTerminal(stream interface{}) (cleanup func(), err error) {
// 	fd, isTerminal := term.GetFdInfo(stream)
// 	if !isTerminal {
// 		return nil, errors.New("not a terminal")
// 	}

// 	state, err := term.SetRawTerminal(fd)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return func() { term.RestoreTerminal(fd, state) }, nil
// }

// setRawTerminalOutput sets the output stream in Windows to raw mode,
// so it disables LF -> CRLF translation.
// It's basically a no-op on unix.
// func setRawTerminalOutput(stream interface{}) (cleanup func(), err error) {
// 	fd, isTerminal := term.GetFdInfo(stream)
// 	if !isTerminal {
// 		return nil, errors.New("not a terminal")
// 	}

// 	state, err := term.SetRawTerminalOutput(fd)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return func() { term.RestoreTerminal(fd, state) }, nil
// }

// // watchTerminalSize watches terminal size changes to propagate to remote tty.
// func watchTerminalSize(out io.Writer, resize chan<- api.TerminalSize) (func(), error) {
// 	fd, isTerminal := term.GetFdInfo(out)
// 	if !isTerminal {
// 		return nil, errors.New("not a terminal")
// 	}

// 	ctx, cancel := context.WithCancel(context.Background())

// 	signalCh := make(chan os.Signal, 1)
// 	setupWindowNotification(signalCh)

// 	sendTerminalSize := func() {
// 		s, err := term.GetWinsize(fd)
// 		if err != nil {
// 			return
// 		}

// 		resize <- api.TerminalSize{
// 			Height: int(s.Height),
// 			Width:  int(s.Width),
// 		}
// 	}
// 	go func() {
// 		for {
// 			select {
// 			case <-ctx.Done():
// 				return
// 			case <-signalCh:
// 				sendTerminalSize()
// 			}
// 		}
// 	}()

// 	go func() {
// 		// send initial size
// 		sendTerminalSize()
// 	}()

// 	return cancel, nil
// }

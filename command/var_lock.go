// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package command

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

type VarLockCommand struct {
	shell     bool
	inFmt     string
	ttl       string
	lockDelay string

	varPutCommand *VarPutCommand
}

func (c *VarLockCommand) Help() string {
	helpText := `
Usage:
nomad var lock [options] <lock spec file reference> child...
nomad var lock [options] <path to store variable> [<variable spec file reference>] child...

  The lock command provides a mechanism for simple distributed locking. A lock
  is created in the given variable, and only when held, is a child process invoked.

  The lock command can be called on an existing variable or an entire new variable
  specification can be provided to the command from a file by using an
  @-prefixed path to a variable specification file. Items to be stored in the 
  variable can be supplied using the specification file as well. 

  Nomad lock launches its children in a shell. By default, Nomad will use the
  shell defined in the environment variable SHELL. If SHELL is not defined, 
  it will default to /bin/sh. It should be noted that not all shells terminate
  child processes when they receive SIGTERM. Under Ubuntu, /bin/sh is linked 
  to dash, which does not terminate its children. In order to ensure that 
  child processes are killed when the lock is lost, be sure to set the SHELL
  environment variable appropriately, or run without a shell by setting -shell=false. 
 
  If ACLs are enabled, this command requires the 'variables:write' capability
  for the destination namespace and path.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Var lock Options:
  -verbose
     Provides additional information via standard error to preserve standard
     output (stdout) for redirected output.

  -ttl
	Optional, TTL for the lock, time the variable will be locked. Defaults to 15s

  -delay
    Optional, time the variable is blocked from locking when a lease is not renewed.
	Defaults to 15s.

  -shell
  	Optional, use a shell to run the command (can set a custom shell via
		the SHELL environment variable). The default value is true.
`
	return strings.TrimSpace(helpText)
}

func (c *VarLockCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.varPutCommand.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{},
	)
}

func (c *VarLockCommand) AutocompleteArgs() complete.Predictor {
	return VariablePathPredictor(c.varPutCommand.Meta.Client)
}

func (c *VarLockCommand) Synopsis() string {
	return "Put a lock on a variable and run a child command if operation is successful"
}

func (c *VarLockCommand) Name() string { return "var lock" }

func (c *VarLockCommand) Run(args []string) int {
	var doVerbose bool
	var err error
	var path string

	flags := c.varPutCommand.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.varPutCommand.Ui.Output(c.Help()) }

	flags.BoolVar(&doVerbose, "verbose", false, "")
	flags.StringVar(&c.ttl, "ttl", "", "Time the variable will be locked")
	flags.StringVar(&c.lockDelay, "delay", "", "Time the variable is blocked from locking when a lease is not renewed")
	flags.BoolVar(&c.shell, "shell", true, "Use a shell to run the command (can set a custom shell via the SHELL "+"environment variable).")

	if fileInfo, _ := os.Stdout.Stat(); (fileInfo.Mode() & os.ModeCharDevice) != 0 {
		flags.StringVar(&c.varPutCommand.outFmt, "out", "none", "")
	} else {
		flags.StringVar(&c.varPutCommand.outFmt, "out", "json", "")
	}

	if err := flags.Parse(args); err != nil {
		c.varPutCommand.Ui.Error(commandErrorText(c))
		return 1
	}

	args = flags.Args()

	// Manage verbose output
	verbose := func(_ string) {} //no-op
	if doVerbose {
		verbose = func(msg string) {
			c.varPutCommand.Ui.Warn(msg)
		}
	}

	c.varPutCommand.verbose = verbose

	if c.varPutCommand.Meta.namespace == "*" {
		c.varPutCommand.Ui.Error(errWildcardNamespaceNotAllowed)
		return 1
	}

	if len(args) < 2 {
		c.varPutCommand.Ui.Error(fmt.Sprintf("Not enough arguments (expected >2, got %d)", len(args)))
		return 1
	}

	path, args, err = c.readPathFromArgs(args)
	if err != nil {
		c.varPutCommand.Ui.Error(err.Error())
		return 1
	}

	sv, err := c.varPutCommand.makeVariable(path)
	if err != nil {
		c.varPutCommand.Ui.Error(fmt.Sprintf("Failed to parse variable data: %s", err))
		return 1
	}

	if sv.Lock == nil {
		if c.ttl == "" && c.lockDelay == "" {
			c.varPutCommand.verbose("Using defaults for the lock")
		}

		sv.Lock = &api.VariableLock{
			TTL:       api.DefaultLockTTL.String(),
			LockDelay: api.DefaultLockDelay.String(),
		}
	}

	if c.ttl != "" {
		c.varPutCommand.verbose("Using TTL for the lock of " + c.ttl)
		_, err := time.ParseDuration(c.ttl)
		if err != nil {
			c.varPutCommand.Ui.Error(fmt.Sprintf("Invalid TTL: %s", err))
			return 1
		}

		sv.Lock.TTL = c.ttl
	}

	if c.lockDelay != "" {
		c.varPutCommand.verbose("Using delay for the lock of " + c.lockDelay)
		_, err := time.ParseDuration(c.ttl)
		if err != nil {
			c.varPutCommand.Ui.Error(fmt.Sprintf("Invalid Lock Delay: %s", err))
			return 1
		}

		sv.Lock.LockDelay = c.lockDelay
	}

	// Get the HTTP client
	client, err := c.varPutCommand.Meta.Client()
	if err != nil {
		c.varPutCommand.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Set up the locks handler
	l, err := client.Locks(api.WriteOptions{}, *sv)
	if err != nil {
		c.varPutCommand.Ui.Error(fmt.Sprintf("Error initializing lock handler: %s", err))
		return 1
	}

	ctx := context.Background()

	// Set up the lease handler
	ll := client.NewLockLeaser(l)
	c.varPutCommand.verbose("Attempting to acquire lock")

	// Run the shell inside the protected function.
	if err := ll.Start(ctx, func(ctx context.Context) error {
		c.varPutCommand.verbose(fmt.Sprintf("Variable locked, ready to execute: %s", args[0]))
		var newCommand func(ctx context.Context, args []string) (*exec.Cmd, error)
		if !c.shell {
			newCommand = subprocess
		} else {
			newCommand = script
		}

		cmd, err := newCommand(ctx, args)
		if err != nil {
			return err
		}

		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		signalCh := make(chan os.Signal, 10)
		signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(signalCh)

		go c.forwardSignals(ctx, cmd, signalCh)

		if err := cmd.Start(); err != nil {
			return err
		}

		return cmd.Wait()

	}); err != nil {
		c.varPutCommand.Ui.Error("Lock error:" + err.Error())
		return 1
	}

	c.varPutCommand.verbose("Releasing the lock")
	return 0
}

func (c *VarLockCommand) readPathFromArgs(args []string) (string, []string, error) {
	var err error
	var path string

	// Handle first argument:  @file or «var path»
	arg := args[0]

	switch {
	case isArgFileRef(arg):
		// ArgFileRefs start with "@" so we need to peel that off
		// detect format based on file extension
		specPath := arg[1:]
		err = c.varPutCommand.setParserForFileArg(specPath)
		if err != nil {
			return "", args, err
		}

		c.varPutCommand.verbose(fmt.Sprintf("Reading whole variable specification from %q", specPath))
		c.varPutCommand.contents, err = os.ReadFile(specPath)
		if err != nil {
			return "", args, fmt.Errorf("Error reading %q: %w", specPath, err)
		}
	default:
		path = sanitizePath(arg)
		c.varPutCommand.verbose(fmt.Sprintf("Writing to path %q", path))
	}

	// Handle second argument: can @file, or child process
	args = args[1:]
	switch {
	case isArgFileRef(args[0]):
		arg := args[0]

		err = c.varPutCommand.setParserForFileArg(arg)
		if err != nil {
			return "", args, err
		}

		c.varPutCommand.verbose(fmt.Sprintf("Creating variable %q from specification file %q", path, arg))
		fPath := arg[1:]
		c.varPutCommand.contents, err = os.ReadFile(fPath)
		if err != nil {
			return "", args, fmt.Errorf("error reading %q: %s", fPath, err)
		}
		args = args[1:]
	default:
		// no-op - should be child process
	}

	return path, args, nil
}

// script returns a command to execute a script through a shell.
func script(ctx context.Context, args []string) (*exec.Cmd, error) {
	shell := "/bin/sh"

	if other := os.Getenv("SHELL"); other != "" {
		shell = other
	}

	return exec.CommandContext(ctx, shell, "-c", strings.Join(args, " ")), nil
}

// subprocess returns a command to execute a subprocess directly.
func subprocess(ctx context.Context, args []string) (*exec.Cmd, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("need an executable to run")
	}
	return exec.CommandContext(ctx, args[0], args[1:]...), nil
}

// ForwardSignals will fire up a goroutine to forward signals to the given
// subprocess until the context is canceled.
func (c *VarLockCommand) forwardSignals(ctx context.Context, cmd *exec.Cmd, sg chan os.Signal) {
	for {
		select {
		case sig := <-sg:
			if err := cmd.Process.Signal(sig); err != nil {
				c.varPutCommand.Ui.Error(fmt.Sprintf("failed to send signal %q: %v", sig, err))
			}

		case <-ctx.Done():
			return
		}
	}
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package shim

import (
	"os"
	"os/exec"
	"time"

	"github.com/hashicorp/nomad/helper/subproc"
	"golang.org/x/sys/unix"
)

const (
	// SubCommand is the first argument to the clone of the nomad agent process
	// for invoking the exec2 driver sandbox shim.
	SubCommand = "exec2-shim"

	// deadline is the total amount of time we allow this process to be alive;
	// in this case we just invoke landlock and then exec into another
	// process
	deadline = 1 * time.Second
)

// init is the entrypoint for the 'nomad e2e-shim' invocation of nomad
//
// The argument format is as follows,
//
// 1. nomad            <- the executable name
// 2. e2e-shim         <- this subcommand
// 3. true/false       <- include default unveil paths
// 4. [mode:path, ...] <- list of additional unveil paths
// 5. --               <- sentinel between following commands
func init() {
	subproc.Do(SubCommand, func() int {
		ctx, cancel := subproc.Context(deadline)
		defer cancel()

		// ensure we die in a timely manner
		subproc.SetExpiration(ctx)

		if n := len(os.Args); n <= 4 {
			subproc.Print("failed to invoke e2e-shim with sufficient args: %d", n)
			return subproc.ExitFailure
		}

		// get the unveil paths and the rest of the command(s) to run
		// from our command arguments
		args := os.Args[3:] // chop off 'nomad e2e-shim <defaults>'
		defaults := os.Args[2] == "true"
		paths, commands := split(args)

		// use landlock to isolate this process and child processes to the
		// set of given filepaths
		if err := lockdown(defaults, paths); err != nil {
			subproc.Print("failed to issue lockdown: %v", err)
			return subproc.ExitFailure
		}

		// locate the absolute path for the task command, as this must be
		// the first argument to the execve(2) call that follows
		cmdpath, err := exec.LookPath(commands[0])
		if err != nil {
			subproc.Print("failed to locate command %q: %v", commands[0], err)
			return subproc.ExitFailure
		}

		// invoke the following commands (nsenter, unshare, the task ...)
		// the environment has already been set for us by the exec2 driver
		//
		// this should never return because this process becomes the
		// invoked command (it is not a child)
		err = unix.Exec(cmdpath, commands, os.Environ())
		if err != nil {
			subproc.Print("failed to exec command %q: %v", cmdpath, err)
			return subproc.ExitFailure
		}

		// should not be possible
		panic("bug: return from exec without error")
	})
}

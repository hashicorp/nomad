// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package shim

import (
	"fmt"
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
// 1. nomad <- the executable name
// 2. e2e-shim <- this subcommand
// 3. true/false <- include default unveil paths
// 4. [mode:path, ...] <- list of additional unveil paths
func init() {
	subproc.Do(SubCommand, func() int {
		fmt.Println("INIT")

		ctx, cancel := subproc.Context(deadline)
		defer cancel()

		// ensure we die in a timely manner
		subproc.SetExpiration(ctx)

		// get the unveil paths and the rest of the command(s) to run
		// from our command arguments
		args := os.Args[3:] // chop off 'nomad e2e-shim bool'
		defaults := os.Args[2] == "true"
		fmt.Println("ARGS", args, "DEFAULTS", defaults)
		paths, commands := split(args)
		fmt.Println("PATHS", paths, "COMMANDS", commands)

		// use landlock to isolate this process and child processes to the
		// set of given filepaths
		if err := lockdown(defaults, paths); err != nil {
			fmt.Println("LOCK FAIL", err)
			return subproc.ExitFailure
		}

		fmt.Println("CMD", commands[0], "CMD_args", commands[1:])

		// locate the absolute path for the task command, as this must be
		// the first argument to the execve(2) call that follows
		cmdpath, err := exec.LookPath(commands[0])
		if err != nil {
			fmt.Println("LOOKPATH failure:", err)
			return subproc.ExitFailure
		}

		// invoke the following commands (nsenter, unshare, the task ...)
		// the environment has already been set for us by the exec2 driver
		fmt.Println("CMDPATH", cmdpath, "CMDARGS", commands)
		err = unix.Exec(cmdpath, commands, os.Environ())

		fmt.Println("EXEC error:", err)

		// the exec did not work and there is nothing to do but die
		panic("bug: failed to exec sandbox commands")
	})
}

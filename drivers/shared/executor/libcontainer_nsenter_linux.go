// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package executor

import (
	"os"
	"runtime"

	"github.com/opencontainers/runc/libcontainer"
	_ "github.com/opencontainers/runc/libcontainer/nsenter"
)

// init is only run on linux and is used when the LibcontainerExecutor starts
// a new process. The libcontainer shim takes over the process, setting up the
// configured isolation and limitions before execve into the user process
//
// This subcommand handler is implemented as an `init`, libcontainer shim is handled anywhere
// this package is used (including tests) without needing to write special command handler.
func init() {
	if len(os.Args) > 1 && os.Args[1] == "libcontainer-shim" {
		runtime.GOMAXPROCS(1)
		runtime.LockOSThread()
		libcontainer.Init()
		panic("--this line should have never been executed, congratulations--")
	}
}

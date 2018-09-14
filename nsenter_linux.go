package main

import (
	"os"
	"runtime"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/opencontainers/runc/libcontainer"
	_ "github.com/opencontainers/runc/libcontainer/nsenter"
)

func init() {
	if len(os.Args) > 1 && os.Args[1] == "libcontainer-shim" {
		runtime.GOMAXPROCS(1)
		runtime.LockOSThread()
		factory, _ := libcontainer.New("")
		if err := factory.StartInitialization(); err != nil {
			hclog.L().Error("failed to initialize libcontainer-shim", "error", err)
		}
		panic("--this line should have never been executed, congratulations--")
	}
}

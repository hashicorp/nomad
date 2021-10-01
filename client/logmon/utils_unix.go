//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris
// +build darwin dragonfly freebsd linux netbsd openbsd solaris

package logmon

import (
	"os"
	"os/signal"
	"runtime/pprof"
	"syscall"

	hclog "github.com/hashicorp/go-hclog"
)

func installPProfHandler(logger hclog.Logger) {
	ch := make(chan os.Signal, 5)
	signal.Notify(ch, syscall.SIGUSR2)
	go func() {
		for range ch {
			emitProfile("heap", logger)
			emitProfile("allocs", logger)
		}
	}()
}

func emitProfile(profile string, logger hclog.Logger) {
	f, err := os.CreateTemp("", "nomad-logmon-pprof-"+profile+"-")
	if err != nil {
		logger.Error("failed to create pprofile file", "err", err)
		return
	}
	defer f.Close()

	err = pprof.Lookup(profile).WriteTo(f, 0)
	if err != nil {
		logger.Error("failed to write heap pprofile", "err", err)
		return
	}

	logger.Info("create pprof file", "profile", profile, "path", f.Name())

}

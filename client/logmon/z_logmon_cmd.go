package logmon

import (
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"syscall"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/plugins/base"
)

// Install a plugin cli handler to ease working with tests
// and external plugins.
// This init() must be initialized last in package required by the child plugin
// process. It's recommended to avoid any other `init()` or inline any necessary calls
// here. See eeaa95d commit message for more details.
func init() {
	if len(os.Args) > 1 && os.Args[1] == "logmon" {

		logger := hclog.New(&hclog.LoggerOptions{
			Level:      hclog.Trace,
			JSONFormat: true,
			Name:       "logmon",
		})

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGUSR2)
		go func() {
			for {
				// note: this will drop any other SIGUSR2 while this is running
				<-sigCh
				runProfile(logger)
			}
		}()

		plugin.Serve(&plugin.ServeConfig{
			HandshakeConfig: base.Handshake,
			Plugins: map[string]plugin.Plugin{
				"logmon": NewPlugin(NewLogMon(logger)),
			},
			GRPCServer: plugin.DefaultGRPCServer,
			Logger:     logger,
		})
		os.Exit(0)
	}
}

func runProfile(logger hclog.Logger) {
	logger.Info("starting logmon profile")
	cpuprofile, err := os.CreateTemp("", "logmon-cpuprofile-*")
	if err != nil {
		logger.Error("could not create cpuprofile tempfile", "error", err)
	}

	memprofile, err := os.CreateTemp("", "logmon-memprofile-*")
	if err != nil {
		logger.Error("could not create cpuprofile tempfile", "error", err)
	}

	defer cpuprofile.Close()
	defer memprofile.Close()

	runtime.GC() // get up-to-date statistics
	if err := pprof.WriteHeapProfile(memprofile); err != nil {
		logger.Error("could not write memory profile", "error", err)
	}

	if err := pprof.StartCPUProfile(cpuprofile); err != nil {
		logger.Error("could not start CPU profile", "error", err)
	}
	time.Sleep(10 * time.Second)
	pprof.StopCPUProfile()

	logger.Info("recorded cpuprofile", "path", cpuprofile.Name())
	logger.Info("recorded memprofile", "path", memprofile.Name())
}

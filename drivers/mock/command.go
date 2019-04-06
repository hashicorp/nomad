package mock

import (
	"errors"
	"io"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	bstructs "github.com/hashicorp/nomad/plugins/base/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
)

func runCommand(c Command, stdout, stderr io.WriteCloser, cancelCh <-chan struct{}, pluginExitTimer <-chan time.Time, logger hclog.Logger) *drivers.ExitResult {
	errCh := make(chan error, 1)

	go runCommandOutput(c, stdout, stderr, cancelCh, logger, errCh)

	timer := time.NewTimer(c.runForDuration)
	defer timer.Stop()

	select {
	case <-timer.C:
		logger.Debug("run_for time elapsed; exiting", "run_for", c.RunFor)
	case <-cancelCh:
		logger.Debug("killed; exiting")
	case <-pluginExitTimer:
		logger.Debug("exiting plugin")
		return &drivers.ExitResult{
			Err: bstructs.ErrPluginShutdown,
		}
	case err := <-errCh:
		logger.Error("error running mock task; exiting", "error", err)
		return &drivers.ExitResult{
			Err: err,
		}
	}

	var exitErr error
	if c.ExitErrMsg != "" {
		exitErr = errors.New(c.ExitErrMsg)
	}

	return &drivers.ExitResult{
		ExitCode: c.ExitCode,
		Signal:   c.ExitSignal,
		Err:      exitErr,
	}
}

func runCommandOutput(c Command, stdout, stderr io.WriteCloser, cancelCh <-chan struct{}, logger hclog.Logger, errCh chan error) {
	defer stdout.Close()
	defer stderr.Close()

	if c.StdoutString == "" {
		return
	}

	if _, err := io.WriteString(stdout, c.StdoutString); err != nil {
		logger.Error("failed to write to stdout", "error", err)
		errCh <- err
		return
	}

	for i := 0; i < c.StdoutRepeat; i++ {
		select {
		case <-cancelCh:
			logger.Warn("exiting before done writing output", "i", i, "total", c.StdoutRepeat)
			return
		case <-time.After(c.stdoutRepeatDuration):
			if _, err := io.WriteString(stdout, c.StdoutString); err != nil {
				logger.Error("failed to write to stdout", "error", err)
				errCh <- err
				return
			}
		}
	}
}

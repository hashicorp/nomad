// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package mock

import (
	"errors"
	"io"
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	bstructs "github.com/hashicorp/nomad/plugins/base/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
)

func runCommand(c Command, stdout, stderr io.WriteCloser, cancelCh <-chan struct{}, pluginExitTimer <-chan time.Time, logger hclog.Logger) *drivers.ExitResult {
	errCh := make(chan error, 1)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		runCommandOutput(stdout, c.StdoutString, c.StdoutRepeat, c.stdoutRepeatDuration, cancelCh, logger, errCh)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		runCommandOutput(stderr, c.StderrString, c.StderrRepeat, c.stderrRepeatDuration, cancelCh, logger, errCh)
	}()

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

	wg.Wait()

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

func runCommandOutput(writer io.WriteCloser,
	output string, outputRepeat int, repeatDuration time.Duration,
	cancelCh <-chan struct{}, logger hclog.Logger, errCh chan error) {

	defer writer.Close()

	if output == "" {
		return
	}

	if _, err := io.WriteString(writer, output); err != nil {
		logger.Error("failed to write to stdout", "error", err)
		errCh <- err
		return
	}

	for i := 0; i < outputRepeat; i++ {
		select {
		case <-cancelCh:
			logger.Warn("exiting before done writing output", "i", i, "total", outputRepeat)
			return
		case <-time.After(repeatDuration):
			if _, err := io.WriteString(writer, output); err != nil {
				logger.Error("failed to write to stdout", "error", err)
				errCh <- err
				return
			}
		}
	}
}

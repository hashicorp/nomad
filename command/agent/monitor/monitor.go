// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package monitor

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"

	log "github.com/hashicorp/go-hclog"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper"
)

// Monitor provides a mechanism to stream logs using go-hclog
// InterceptLogger and SinkAdapter. It allows streaming of logs
// at a different log level than what is set on the logger.
type Monitor interface {
	// Start returns a channel of log messages which are sent
	// ever time a log message occurs
	Start() <-chan []byte

	// Stop de-registers the sink from the InterceptLogger
	// and closes the log channels
	Stop()

	// MonitorExternal returns a channel of monitor/exernal messages
	MonitorExternal(opts *cstructs.MonitorExternalRequest) <-chan []byte
}

// monitor implements the Monitor interface
type monitor struct {
	// protects droppedCount and logCh
	sync.Mutex

	sink log.SinkAdapter

	// logger is the logger we will be monitoring
	logger log.InterceptLogger

	// logCh is a buffered chan where we send logs when streaming
	logCh chan []byte

	// doneCh coordinates the shutdown of logCh
	doneCh chan struct{}

	// droppedCount is the current count of messages
	// that were dropped from the logCh buffer.
	// only access under lock
	droppedCount int
	bufSize      int
	// droppedDuration is the amount of time we should
	// wait to check for dropped messages. Defaults
	// to 3 seconds
	droppedDuration time.Duration
}

// New creates a new Monitor. Start must be called in order to actually start
// streaming logs
func New(buf int, logger log.InterceptLogger, opts *log.LoggerOptions) Monitor {
	return new(buf, logger, opts)
}

func new(buf int, logger log.InterceptLogger, opts *log.LoggerOptions) *monitor {
	sw := &monitor{
		logger:          logger,
		logCh:           make(chan []byte, buf),
		doneCh:          make(chan struct{}, 1),
		bufSize:         buf,
		droppedDuration: 3 * time.Second,
	}

	opts.Output = sw
	sink := log.NewSinkAdapter(opts)
	sw.sink = sink

	return sw
}

// Stop deregisters the sink and stops the monitoring process
func (d *monitor) Stop() {
	d.logger.DeregisterSink(d.sink)
	close(d.doneCh)
}

// Start registers a sink on the monitor's logger and starts sending
// received log messages over the returned channel.
func (d *monitor) Start() <-chan []byte {
	// register our sink with the logger
	d.logger.RegisterSink(d.sink)

	streamCh := make(chan []byte, d.bufSize)

	// run a go routine that listens for streamed
	// log messages and sends them to streamCh
	go func() {
		defer close(streamCh)

		for {
			select {
			case log := <-d.logCh:
				select {
				case <-d.doneCh:
					return
				case streamCh <- log:
				}
			case <-d.doneCh:
				return
			}
		}
	}()

	// run a go routine that periodically checks for
	// dropped messages and makes room on the logCh
	// to add a dropped message count warning
	go func() {
		timer, stop := helper.NewSafeTimer(d.droppedDuration)
		defer stop()

		// loop and check for dropped messages
		for {
			timer.Reset(d.droppedDuration)

			select {
			case <-d.doneCh:
				return
			case <-timer.C:
				d.Lock()

				// Check if there have been any dropped messages.
				if d.droppedCount > 0 {
					dropped := fmt.Sprintf("[WARN] Monitor dropped %d logs during monitor request\n", d.droppedCount)
					select {
					case <-d.doneCh:
						d.Unlock()
						return
					// Try sending dropped message count to logCh in case
					// there is room in the buffer now.
					case d.logCh <- []byte(dropped):
					default:
						// Drop a log message to make room for "Monitor dropped.." message
						select {
						case <-d.logCh:
							d.droppedCount++
							dropped = fmt.Sprintf("[WARN] Monitor dropped %d logs during monitor request\n", d.droppedCount)
						default:
						}
						d.logCh <- []byte(dropped)
					}
					d.droppedCount = 0
				}
				// unlock after handling dropped message
				d.Unlock()
			}
		}
	}()

	return streamCh
}

// Write attempts to send latest log to logCh
// it drops the log if channel is unavailable to receive
func (d *monitor) Write(p []byte) (n int, err error) {
	d.Lock()
	defer d.Unlock()

	// ensure logCh is still open
	select {
	case <-d.doneCh:
		return
	default:
	}

	bytes := make([]byte, len(p))
	copy(bytes, p)

	select {
	case d.logCh <- bytes:
	default:
		d.droppedCount++
	}

	return len(p), nil
}

// MonitorExternal reads a file or executes a CLI command and streams a single
// log bundle over the monitor's channel
func (d *monitor) MonitorExternal(opts *cstructs.MonitorExternalRequest) <-chan []byte {

	//if runtime.GOOS != "linux" && opts.ServiceName != "" {
	//	d.logger.Error("systemd unit log monitoring only available on linux")
	//	return nil
	//}

	// Double checking options are as expected
	if len(opts.ServiceName) == 0 && len(opts.LogPath) == 0 {
		d.logger.Error("serviceName or logPath must be set")
		return nil
	} else if len(opts.ServiceName) != 0 && len(opts.LogPath) != 0 {
		d.logger.Error("both serviceName and logPath cannot be set")
		return nil
	}
	var (
		multiReader io.Reader
		cmd         *exec.Cmd
		prepErr     error
		useCli      bool
	)
	if len(opts.ServiceName) != 0 && len(opts.LogPath) == 0 {
		useCli = true
		cmd, multiReader, prepErr = d.cliReader(opts)
		cmd.Start()
	} else if len(opts.ServiceName) == 0 && len(opts.LogPath) != 0 {
		useCli = false
		multiReader, prepErr = d.fileReader(opts)

	}

	if prepErr != nil {
		d.logger.Error("error attempting to prepare cli command", "error", prepErr.Error())
	}

	streamCh := make(chan []byte)
	// Read, copy, and send to channel until we hit EOF or error
	go func() {
		if useCli {
			defer cmd.Wait()
		}
		defer close(streamCh)
		logChunk := make([]byte, 32)

		for {
			n, readErr := multiReader.Read(logChunk)
			if readErr != nil && readErr != io.EOF {
				d.logger.Error("unable to read logs into channel", readErr.Error())
				return
			}

			streamCh <- logChunk[:n]

			if readErr == io.EOF {
				break
			}

		}
	}()
	return streamCh
}
func (d *monitor) cliReader(opts *cstructs.MonitorExternalRequest) (*exec.Cmd, io.Reader, error) {
	const defaultDuration = "72"
	var cmdString string

	cmdDuration := opts.LogSince
	// Set logSince to default if unset by caller
	if opts.LogSince == "0" {
		cmdDuration = defaultDuration
	}

	// build command

	cmdString = fmt.Sprintf("journalctl -xe -u %s --no-pager --since '%s hours ago'", opts.ServiceName, cmdDuration)
	if opts.Follow {
		cmdString = cmdString + " -f"
	}
	shell := "/bin/sh"
	if other := os.Getenv("SHELL"); other != "" {
		shell = other
	}

	// We aren't exposing opt.TstFile in the CLI and require it be running in a
	// testing environment but I can pull it if it still feels risky to have
	// in the RPC params
	if opts.TstFile != "" && testing.Testing() {
		cmdString = fmt.Sprintf("cat %s", opts.TstFile)
	} else if opts.TstFile != "" && !testing.Testing() {
		//temporary iteration helper to enable me to test non cmdString elements from mac
		cmdString = "cat /Users/tehut/go/src/github.com/hashicorp/nomad/t3.txt"
	}

	cmd := exec.CommandContext(context.Background(), shell, "-c", cmdString)

	// set up reader
	stdOut, err := cmd.StdoutPipe()
	if err != nil {
		d.logger.Error("unable to read logs into buffer", err.Error())
		return nil, nil, err
	}
	stdErr, err := cmd.StderrPipe()
	if err != nil {
		d.logger.Error("unable to read logs into buffer", err.Error())
		return nil, nil, err
	}
	multiReader := io.MultiReader(stdOut, stdErr)

	return cmd, multiReader, nil
}

func (d *monitor) fileReader(opts *cstructs.MonitorExternalRequest) (io.Reader, error) {
	file, err := os.Open(opts.LogPath)
	if err != nil {
		d.logger.Error("failed to open log file", "error", err.Error())
	}

	return file, nil
}

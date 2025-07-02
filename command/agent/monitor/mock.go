// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package monitor

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"
)

// ExportMonitor implements the Monitor interface for testing
type ExportMonitor struct {
	// logCh is a buffered chan where we send logs when streaming
	LogCh chan []byte

	// DoneCh coordinates the shutdown of logCh
	DoneCh chan struct{}

	// droppedCount is the current count of messages
	// that were dropped from the logCh buffer.
	// only access under lock
	droppedCount int

	bufSize int
	// droppedDuration is the amount of time we should
	// wait to check for dropped messages. Defaults
	// to 3 seconds
	droppedDuration time.Duration
}

const bufSize = 512

func Mock() ExportMonitor {
	sw := ExportMonitor{
		LogCh:           make(chan []byte, bufSize),
		DoneCh:          make(chan struct{}, 1),
		bufSize:         bufSize,
		droppedDuration: 3 * time.Second,
	}

	return sw
}

// Stop stops the monitoring process
func (d ExportMonitor) Stop() {
	close(d.DoneCh)
}

// Start registers a sink on the monitor's logger and starts sending
// received log messages over the returned channel.
func (d ExportMonitor) Start() <-chan []byte {
	// noop to satisfy interface
	streamCh := make(chan []byte, d.bufSize)

	return streamCh
}

// Write attempts to send latest log to logCh
// it drops the log if channel is unavailable to receive
func (d ExportMonitor) Write(p []byte) (n int, err error) {
	select {
	case <-d.DoneCh:
		return
	default:
	}

	bytes := make([]byte, len(p))
	copy(bytes, p)

	select {
	case d.LogCh <- bytes:
	default:
		d.droppedCount++
	}

	return len(p), nil
}

// MonitorExport reads a file or executes a CLI command and streams a single
// log bundle over the monitor's channel
func (d ExportMonitor) MonitorExport(opts MonitorExportOpts) <-chan []byte {
	var (
		multiReader io.Reader
		cmd         *exec.Cmd
		prepErr     error
		useCli      bool
	)

	if opts.OnDisk {
		multiReader, prepErr = d.fileReader(opts.NomadLogPath)
		if prepErr != nil {
			fmt.Printf("error attempting to prepare reader %s", prepErr.Error())
			return nil
		}
	} else {
		useCli = true
		cmd, multiReader, prepErr = d.cliReader(opts)
		if prepErr != nil {
			fmt.Printf("error attempting to prepare reader %s", prepErr.Error())
			return nil
		}
		cmd.Start()
	}

	// Read, copy, and send to channel until we hit EOF or error
	streamCh := make(chan []byte)
	go func() {
		if useCli {
			defer cmd.Wait()
		}
		defer close(streamCh)
		logChunk := make([]byte, 32)

		for {
			n, readErr := multiReader.Read(logChunk)
			if readErr != nil && readErr != io.EOF {
				fmt.Printf("error attempting to read logs into channel %s", readErr.Error())
				return
			}

			streamCh <- logChunk[:n]

			if readErr == io.EOF && !opts.Follow {
				break
			}
		}
	}()
	return streamCh
}
func (d ExportMonitor) cliReader(opts MonitorExportOpts) (*exec.Cmd, io.Reader, error) {
	var cmdString string
	// Vet servicename again
	if err := ScanServiceName(opts.ServiceName); err != nil {
		return nil, nil, err
	}

	// build command with vetted inputs
	shell := "/bin/sh"
	if other := os.Getenv("SHELL"); other != "" {
		shell = other
	}

	if opts.Follow {
		cmdString = "less" + opts.NomadLogPath
	} else {
		cmdString = "cat " + opts.NomadLogPath
	}
	cmd := exec.CommandContext(context.Background(), shell, "-c", cmdString)

	// set up reader
	stdOut, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	stdErr, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, err
	}
	multiReader := io.MultiReader(stdOut, stdErr)

	return cmd, multiReader, nil
}

func (d ExportMonitor) fileReader(logfile string) (io.Reader, error) {
	file, err := os.Open(logfile)
	if err != nil {
		return nil, err
	}

	return file, nil
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package monitor

import (
	"errors"
	"io"
	"os/exec"
	"runtime"

	"github.com/hashicorp/go-hclog"
)

// ExportMonitor implements the Monitor interface for testing
type ExportMonitor struct {
	logger hclog.Logger

	// logCh is a buffered chan where we send logs when streaming
	LogCh chan []byte

	// DoneCh coordinates the shutdown of logCh
	DoneCh chan struct{}

	bufSize int

	// ExportReader can read from the cli or the NomadFilePath
	ExportReader ExportReader
}

type MonitorExportOpts struct {
	Logger hclog.Logger

	// LogsSince sets the lookback time for monitorExport logs in hours
	LogSince string

	// OnDisk indicates that nomad should export logs written to the configured nomad log path
	OnDisk bool

	// ServiceName is the systemd service for which we want to retrieve logs
	// Cannot be used with OnDisk
	ServiceName string

	// NomadLogPath is set to the nomad log path by the HTTP agent if OnDisk
	// is true
	NomadLogPath string

	// Follow indicates that the monitor should continue to deliver logs until
	// an outside interrupt
	Follow bool
}

const bufSize = 512

type ExportReader struct {
	io.Reader
	Cmd    *exec.Cmd
	UseCli bool
	Follow bool
}

func NewExportMonitor(opts MonitorExportOpts) (*ExportMonitor, error) {
	var (
		multiReader io.Reader
		cmd         *exec.Cmd
		prepErr     error
	)

	ExportReader := ExportReader{Follow: opts.Follow}

	if runtime.GOOS != "linux" &&
		opts.ServiceName != "" {
		return nil, errors.New("journald log monitoring only available on linux")
	}

	if opts.OnDisk {
		multiReader, prepErr = fileReader(opts.NomadLogPath)
		if prepErr != nil {
			return nil, prepErr
		}

		ExportReader.Reader = multiReader
		ExportReader.UseCli = false
	} else {
		cmd, multiReader, prepErr = cliReader(opts)
		if prepErr != nil {
			return nil, prepErr
		}

		ExportReader.Reader = multiReader
		ExportReader.Cmd = cmd
		ExportReader.UseCli = true
	}
	sw := ExportMonitor{
		logger:       opts.Logger,
		LogCh:        make(chan []byte, bufSize),
		DoneCh:       make(chan struct{}, 1),
		bufSize:      bufSize,
		ExportReader: ExportReader,
	}

	return &sw, nil
}

// Stop stops the monitoring process
func (d *ExportMonitor) Stop() {
	close(d.DoneCh)
}

// Start registers a sink on the monitor's logger and starts sending
// received log messages over the returned channel.
func (d *ExportMonitor) Start() <-chan []byte {
	var (
		cmd    *exec.Cmd
		useCli bool
	)

	if d.ExportReader.UseCli {
		useCli = true
		cmd = d.ExportReader.Cmd
	}
	// Read, copy, and send to channel until we hit EOF or error
	streamCh := make(chan []byte)
	go func() {
		if useCli {
			defer cmd.Wait()
		}
		defer close(streamCh)
		logChunk := make([]byte, bufSize)
	OUTER:
		for {
			select {
			case <-d.DoneCh:
				break OUTER
			default:
				n, readErr := d.ExportReader.Read(logChunk)
				if readErr != nil && readErr != io.EOF {
					d.logger.Error("unable to read logs into channel", readErr.Error())
					return
				}

				streamCh <- logChunk[:n]

				if readErr == io.EOF && !d.ExportReader.Follow {
					break OUTER
				}
			}
		}
	}()
	return streamCh
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package monitor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/hashicorp/go-hclog"
)

// ExportMonitor implements the Monitor interface for testing
type ExportMonitor struct {
	logger hclog.Logger

	// DoneCh coordinates breaking out of the export loop
	DoneCh chan struct{}

	bufSize int

	// ExportReader can read from the cli or the NomadFilePath
	ExportReader ExportReader
}

type MonitorExportOpts struct {
	Logger hclog.Logger

	// LogsSince sets the lookback time for monitorExport logs in hours
	LogsSince string

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

	// Context passed from client to close the cmd and exit the function
	Context context.Context
}

const bufSize = 512

type ExportReader struct {
	io.Reader
	Cmd    *exec.Cmd
	UseCli bool
	Follow bool
}

// NewExportMonitor validates and prepares the appropriate reader before
// returning a new ExportMonitor or the appropriate error
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
		cmd.Start()
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
				if readErr == io.EOF && n == 0 && !d.ExportReader.Follow {
					break OUTER
				}
			}
		}
	}()
	return streamCh
}

func cliReader(opts MonitorExportOpts) (*exec.Cmd, io.Reader, error) {
	// Vet servicename again
	if err := ScanServiceName(opts.ServiceName); err != nil {
		return nil, nil, err
	}
	cmdDuration := "72 hours"
	if opts.LogsSince != "" {
		parsedDur, err := time.ParseDuration(opts.LogsSince)
		if err != nil {
			return nil, nil, err
		}
		cmdDuration = parsedDur.String()
	}
	// build command with vetted inputs
	cmdArgs := []string{"-xu", opts.ServiceName, "--since", fmt.Sprintf("%s ago", cmdDuration)}

	if opts.Follow {
		cmdArgs = append(cmdArgs, "-f")
	}
	cmd := exec.CommandContext(opts.Context, "journalctl", cmdArgs...)

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

func fileReader(logPath string) (io.Reader, error) {
	file, err := os.Open(logPath)
	if err != nil {
		return nil, err
	}

	return file, nil
}

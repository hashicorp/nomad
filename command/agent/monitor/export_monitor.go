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
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
)

// ExportMonitor implements the Monitor interface for testing
type ExportMonitor struct {
	logger hclog.Logger

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

	// Putting the export opts on the monitor allows us to maintain the monitor interface
	Opts MonitorExportOpts
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

func NewExportMonitor(opts MonitorExportOpts) *ExportMonitor {
	sw := ExportMonitor{
		logger:          opts.Logger,
		LogCh:           make(chan []byte, bufSize),
		DoneCh:          make(chan struct{}, 1),
		bufSize:         bufSize,
		droppedDuration: 3 * time.Second,
		Opts:            opts,
	}

	return &sw
}

// Stop stops the monitoring process
func (d *ExportMonitor) Stop() {
	close(d.DoneCh)
}

// Start registers a sink on the monitor's logger and starts sending
// received log messages over the returned channel.
func (d *ExportMonitor) Start() <-chan []byte {
	var (
		multiReader io.Reader
		cmd         *exec.Cmd
		prepErr     error
		useCli      bool
	)

	if runtime.GOOS != "linux" &&
		d.Opts.ServiceName != "" {
		d.logger.Error("systemd unit log monitoring only available on linux")
		return nil
	}

	if d.Opts.OnDisk {
		multiReader, prepErr = d.fileReader()
		if prepErr != nil {
			d.logger.Error("error attempting to prepare reader", "error", prepErr.Error())
			return nil
		}
	} else {
		useCli = true
		cmd, multiReader, prepErr = d.cliReader()
		if prepErr != nil {
			d.logger.Error("error attempting to prepare reader", "error", prepErr.Error())
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
				d.logger.Error("unable to read logs into channel", readErr.Error())
				return
			}

			streamCh <- logChunk[:n]

			if readErr == io.EOF && !d.Opts.Follow {
				break
			}
		}
	}()
	return streamCh
}

// Write attempts to send latest log to logCh
// it drops the log if channel is unavailable to receive
func (d *ExportMonitor) Write(p []byte) (n int, err error) {
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

func (d *ExportMonitor) cliReader() (*exec.Cmd, io.Reader, error) {

	// Vet servicename again
	if err := ScanServiceName(d.Opts.ServiceName); err != nil {
		return nil, nil, err
	}
	cmdDuration := "72 hours"
	if d.Opts.LogSince != "" {
		parsedDur, err := time.ParseDuration(d.Opts.LogSince)
		if err != nil {
			return nil, nil, err
		}
		cmdDuration = parsedDur.String()
	}
	// build command with vetted inputs
	cmdArgs := strings.Join([]string{"-xu", d.Opts.ServiceName, "--since", fmt.Sprintf("%s ago", cmdDuration)}, " ")

	if d.Opts.Follow {
		cmdArgs += "- f"
	}
	cmd := exec.CommandContext(context.Background(), "journalctl", cmdArgs)

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

func (d *ExportMonitor) fileReader() (io.Reader, error) {
	file, err := os.Open(d.Opts.NomadLogPath)
	if err != nil {
		return nil, err
	}

	return file, nil
}

func ScanServiceName(input string) error {
	input = strings.TrimSpace(input)
	// only allow
	// ":", "-", "_", ".", "\" , "@" (only difference from above)
	re := regexp.MustCompile(`\w*[.:_\-\\@]\w*`)

	safe := re.MatchString(input)
	if !safe {
		return errors.New("service name must conform to systemd conventions")
	}
	return nil
}

func ScanField(input string, fieldname string) error {
	input = strings.TrimSpace(input)
	// only allow
	// ":", "-", "_", ".", "\" , and "/" (replacing @ with / is only difference from above)
	re := regexp.MustCompile(`\w*[_\-\\/]\w*`)

	safe := re.MatchString(input)
	if !safe {
		return errors.New(fmt.Errorf("invalid character detected in %s value", fieldname).Error())
	}
	return nil
}

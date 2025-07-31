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
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
)

const defaultBufSize = 512

// ExportMonitor implements the Monitor interface for testing
type ExportMonitor struct {
	sync.Mutex

	logCh  chan []byte
	logger hclog.Logger

	// doneCh coordinates breaking out of the export loop
	doneCh chan struct{}

	// ExportReader can read from the cli or the NomadFilePath
	ExportReader *ExportReader

	bufSize int
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

	// ExportMonitor's buffer size, defaults to 512 if unset by caller
	BufSize int
}

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
		exportReader *ExportReader
		bufSize      int
	)

	if runtime.GOOS != "linux" &&
		opts.ServiceName != "" {
		return nil, errors.New("journald log monitoring only available on linux")
	}

	if opts.BufSize == 0 {
		bufSize = defaultBufSize
	} else {
		bufSize = opts.BufSize
	}

	if opts.OnDisk && opts.ServiceName == "" {
		e, prepErr := fileReader(opts)
		if prepErr != nil {
			return nil, prepErr
		}
		exportReader = e
	}

	if opts.ServiceName != "" && !opts.OnDisk {
		e, prepErr := cliReader(opts)
		if prepErr != nil {
			return nil, prepErr
		}
		exportReader = e
	}

	sw := ExportMonitor{
		logger:       hclog.Default().Named("export"),
		doneCh:       make(chan struct{}, 1),
		logCh:        make(chan []byte, bufSize),
		bufSize:      bufSize,
		ExportReader: exportReader,
	}

	return &sw, nil
}

// ScanServiceName checks that the length, prefix and suffix conform to
// systemd conventions and ensures the service name includes the word 'nomad'
func ScanServiceName(input string) error {
	prefix := ""
	// invalid if prefix and suffix together are > 255 char
	if len(input) > 255 {
		return errors.New("service name too long")
	}

	if isNomad := strings.Contains(input, "nomad"); !isNomad {
		return errors.New(`service name must include 'nomad`)
	}

	// if there is a suffix, check against list of valid suffixes
	// and set prefix to exclude suffix index, else set prefix
	splitInput := strings.Split(input, ".")
	if len(splitInput) < 2 {
		prefix = input
	} else {
		suffix := splitInput[len(splitInput)-1]
		validSuffix := []string{
			"service",
			"socket",
			"device",
			"mount",
			"automount",
			"swap",
			"target",
			"path",
			"timer",
			"slice",
			"scope",
		}
		if valid := slices.Contains(validSuffix, suffix); !valid {
			return errors.New("invalid suffix")
		}
		prefix = strings.Join(splitInput[:len(splitInput)-1], "")
	}

	safe, _ := regexp.MatchString(`^[\w\\._-]*(@[\w\\._-]+)?$`, prefix)
	if !safe {
		return fmt.Errorf("%s does not meet systemd conventions", prefix)
	}
	return nil
}

func cliReader(opts MonitorExportOpts) (*ExportReader, error) {
	isCli := true
	// Vet servicename again
	if err := ScanServiceName(opts.ServiceName); err != nil {
		return nil, err
	}
	cmdDuration := "72 hours"
	if opts.LogsSince != "" {
		parsedDur, err := time.ParseDuration(opts.LogsSince)
		if err != nil {
			return nil, err
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
		return nil, err
	}
	stdErr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	multiReader := io.MultiReader(stdOut, stdErr)
	cmd.Start()

	return &ExportReader{multiReader, cmd, isCli, opts.Follow}, nil
}

func fileReader(opts MonitorExportOpts) (*ExportReader, error) {
	notCli := false
	file, err := os.Open(opts.NomadLogPath)
	if err != nil {
		return nil, err
	}
	return &ExportReader{file, nil, notCli, opts.Follow}, nil

}

// Stop stops the monitoring process
func (d *ExportMonitor) Stop() {
	select {
	case _, ok := <-d.doneCh:
		if !ok {
			if d.ExportReader.UseCli {
				d.ExportReader.Cmd.Wait()
			}
			close(d.logCh)
			return
		}
	default:
	}
	close(d.logCh)
}

// Start reads data from the monitor's ExportReader into its logCh
func (d *ExportMonitor) Start() <-chan []byte {
	// Read, copy, and send to channel until we hit EOF or error
	go func() {
		defer d.Stop()
		logChunk := make([]byte, d.bufSize)

		for {
			n, readErr := d.ExportReader.Read(logChunk)
			if readErr != nil && readErr != io.EOF {
				d.logger.Error("unable to read logs into channel", readErr.Error())
				return
			}

			d.Write(logChunk[:n])

			if readErr == io.EOF {
				break
			}
		}
		close(d.doneCh)
	}()
	return d.logCh
}

// Write attempts to send latest log to logCh
// it drops the log if channel is unavailable to receive
func (d *ExportMonitor) Write(p []byte) (n int) {
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

	d.logCh <- bytes

	return len(p)
}

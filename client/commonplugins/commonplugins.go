// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package commonplugins

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"syscall"
	"time"

	"github.com/hashicorp/go-version"
)

const (
	CPI_TIMEOUT_SOFT time.Duration = 2 * time.Second
	CPI_TIMEOUT_HARD time.Duration = 1 * time.Second
)

var (
	ErrPluginNotExists     error = errors.New("plugin not found")
	ErrPluginNotExecutable error = errors.New("plugin not executable")
)

type CommonPlugin interface {
	Fingerprint(ctx context.Context) (*PluginFingerprint, error)
}

// CommonPlugins are expected to respond to 'fingerprint' calls with json that
// unmarshals to this struct.
type PluginFingerprint struct {
	Version *version.Version `json:"version"`
	Type    *string          `json:"type"`
}

// runPlugin is a helper for executing the provided Cmd and capturing stdout/stderr.
// This helper implements both the soft and hard timeouts defined by the common
// plugins interface.
func runPlugin(cmd *exec.Cmd) (stdout, stderr []byte, err error) {
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &outBuf, &errBuf

	// Start the command
	if err = cmd.Start(); err != nil {
		return
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	timer := time.NewTimer(CPI_TIMEOUT_SOFT)
	defer timer.Stop()

	select {
	case <-timer.C:
		// Soft timeout
		_ = cmd.Process.Signal(syscall.SIGTERM)
		killTimer := time.NewTimer(CPI_TIMEOUT_HARD)
		defer killTimer.Stop()

		select {
		case <-killTimer.C:
			// Hard timeout
			// TODO: should we wait until this process has exited?
			err = cmd.Process.Kill()
		case err = <-done:
		}
	case err = <-done:
	}

	stdout = outBuf.Bytes()
	stderr = errBuf.Bytes()
	return
}

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
func runPlugin(cmd *exec.Cmd, killTimeout time.Duration) (stdout, stderr []byte, err error) {
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf

	done := make(chan error, 1)
	cmd.Cancel = func() error {
		var err error

		_ = cmd.Process.Signal(syscall.SIGTERM)
		killTimer := time.NewTimer(killTimeout)
		defer killTimer.Stop()

		select {
		case <-killTimer.C:
			err = cmd.Process.Kill()
		case err = <-done:
		}

		return err
	}

	// start the command
	stdout, err = cmd.Output()
	done <- err

	stderr = errBuf.Bytes()
	return
}

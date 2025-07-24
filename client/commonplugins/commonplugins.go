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
func runPlugin(ctx context.Context, cmd *exec.Cmd, cmdTimeout, killTimeout time.Duration) (stdout, stderr []byte, err error) {
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

	plugCtx, cancel := context.WithTimeout(ctx, cmdTimeout)
	defer cancel()

	select {
	case <-plugCtx.Done():
		err = cmd.Process.Signal(syscall.SIGTERM)
		killTimer := time.NewTimer(killTimeout)
		defer killTimer.Stop()

		select {
		case <-killTimer.C:
			// Hard timeout
			err = cmd.Process.Kill()
		case err = <-done:
		}
	case err = <-done:
	}

	stdout = outBuf.Bytes()
	stderr = errBuf.Bytes()
	return
}

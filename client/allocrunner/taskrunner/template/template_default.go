// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !windows

package template

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"time"

	"github.com/hashicorp/nomad/client/allocrunner/taskrunner/template/renderer"
)

// createPlatformSandbox is a no-op outside of windows
func createPlatformSandbox(_ *TaskTemplateManagerConfig) error { return nil }

// destroyPlatformSandbox is a no-op outside of windows
func destroyPlatformSandbox(_ *TaskTemplateManagerConfig) error { return nil }

// renderTemplateInSandbox runs the template-render command in a subprocess that
// will chroot itself to prevent a task from swapping a directory between the
// sandbox path and the destination with a symlink pointing to somewhere outside
// the sandbox.
//
// See renderer/ subdirectory for implementation.
func renderTemplateInSandbox(cfg *sandboxConfig) (string, int, error) {

	// Safe to inject user input as command arguments since Go's exec.Command
	// does not invoke a shell.
	args := []string{
		"template-render",
		"write",
		"-sandbox-path", cfg.sandboxPath,
		"-dest-path", cfg.destPath,
		"-perms", cfg.perms,
	}
	if cfg.user != "" {
		args = append(args, "-user")
		args = append(args, cfg.user)
	}
	if cfg.group != "" {
		args = append(args, "-group")
		args = append(args, cfg.group)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// note: we can't simply set cmd.SysProcAttr.Chroot here because the Nomad
	// binary isn't in the chroot
	cmd := exec.CommandContext(ctx, cfg.thisBin, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", 1, err
	}

	go func() {
		defer stdin.Close()
		io.Copy(stdin, bytes.NewReader(cfg.contents))
	}()

	out, err := cmd.CombinedOutput()
	code := cmd.ProcessState.ExitCode()
	if code == renderer.ExitWouldRenderButDidnt {
		err = nil // erase the "exit code 117" error
	}

	return string(out), code, err
}

// readTemplateFromSandbox runs the template-render command in a subprocess that
// will chroot itself to prevent a task from swapping a directory between the
// sandbox path and the source with a symlink pointing to somewhere outside
// the sandbox.
func readTemplateFromSandbox(cfg *sandboxConfig) ([]byte, []byte, int, error) {

	// Safe to inject user input as command arguments since Go's exec.Command
	// does not invoke a shell. Also, the only user-controlled argument here is
	// the source path which we've already verified is at least a valid path in
	// the caller.
	args := []string{
		"template-render",
		"read",
		"-sandbox-path", cfg.sandboxPath,
		"-source-path", cfg.sourcePath,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// note: we can't simply set cmd.SysProcAttr.Chroot here because the Nomad
	// binary isn't in the chroot
	cmd := exec.CommandContext(ctx, cfg.thisBin, args...)
	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb

	err := cmd.Run()
	stdout := outb.Bytes()
	stderr := errb.Bytes()
	return stdout, stderr, cmd.ProcessState.ExitCode(), err
}

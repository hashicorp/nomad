// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build windows

package template

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/taskrunner/template/renderer"
	"github.com/hashicorp/nomad/helper/subproc"
	"github.com/hashicorp/nomad/helper/winappcontainer"
	"github.com/hashicorp/nomad/helper/winexec"
)

const ExitCodeFatal int = 13 // typically this is going to be a bug in Nomad

// createPlatformSandbox creates the AppContainer profile and sets DACLs on the
// files we want to grant access to.
func createPlatformSandbox(cfg *TaskTemplateManagerConfig) error {

	if !isSandboxEnabled(cfg) {
		return nil
	}
	thisBin := subproc.Self()

	containerCfg := &winappcontainer.AppContainerConfig{
		Name: cfg.TaskID,
		AllowedPaths: []string{
			thisBin,
			filepath.Dir(cfg.TaskDir), // give access to the whole alloc working directory
		},
	}
	if cfg.Logger == nil {
		cfg.Logger = hclog.Default() // prevents panics in tests
	}

	err := winappcontainer.CreateAppContainer(cfg.Logger, containerCfg)
	if err != nil {
		// if Nomad is running as an unprivileged user, we might not be able to
		// create the sandbox, but in that case we're not vulnerable to the
		// attacks this is intended to prevent anyways
		if errors.Is(err, winappcontainer.ErrAccessDeniedToCreateSandbox) {
			cfg.Logger.Debug("could not create platform sandbox", "error", err)
			return nil
		}
		return fmt.Errorf("could not create platform sandbox: %w", err)
	}

	return nil
}

// destroyPlatformSandbox deletes the AppContainer profile.
func destroyPlatformSandbox(cfg *TaskTemplateManagerConfig) error {

	if cfg.ClientConfig.TemplateConfig.DisableSandbox {
		return nil
	}

	if cfg.Logger == nil {
		cfg.Logger = hclog.Default()
	}

	err := winappcontainer.DeleteAppContainer(cfg.Logger, cfg.TaskID)
	if err != nil {
		cfg.Logger.Warn("could not destroy platform sandbox", "error", err)
	}
	return err
}

// renderTemplateInSandbox runs the template-render command in an AppContainer to
// prevent a task from swapping a directory between the sandbox path and the
// destination with a symlink pointing to somewhere outside the sandbox.
//
// See renderer/ subdirectory for implementation.
func renderTemplateInSandbox(cfg *sandboxConfig) (string, int, error) {
	procThreadAttrs, cleanup, err := winappcontainer.CreateProcThreadAttributes(cfg.taskID)
	if err != nil {
		return "", ExitCodeFatal, fmt.Errorf("could not create proc attributes: %v", err)
	}
	defer cleanup()

	// Safe to inject user input as command arguments since winexec.Command
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

	cmd := winexec.CommandContext(ctx, cfg.thisBin, args...)
	cmd.ProcThreadAttributes = procThreadAttrs

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
	procThreadAttrs, cleanup, err := winappcontainer.CreateProcThreadAttributes(cfg.taskID)
	if err != nil {
		return nil, nil, ExitCodeFatal, fmt.Errorf("could not create proc attributes: %v", err)
	}
	defer cleanup()

	// Safe to inject user input as command arguments since winexec.Command
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

	cmd := winexec.CommandContext(ctx, cfg.thisBin, args...)
	cmd.ProcThreadAttributes = procThreadAttrs
	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb

	err = cmd.Run()
	stdout := outb.Bytes()
	stderr := errb.Bytes()
	return stdout, stderr, cmd.ProcessState.ExitCode(), err
}

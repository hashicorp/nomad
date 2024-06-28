// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !windows

package template

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/hashicorp/consul-template/renderer"
	trenderer "github.com/hashicorp/nomad/client/allocrunner/taskrunner/template/renderer"
	"github.com/hashicorp/nomad/helper/subproc"
)

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
	if code == trenderer.ExitWouldRenderButDidnt {
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

func RenderFn(taskID, taskDir string, sandboxEnabled bool) func(*renderer.RenderInput) (*renderer.RenderResult, error) {
	if !sandboxEnabled {
		return nil
	}
	thisBin := subproc.Self()

	return func(i *renderer.RenderInput) (*renderer.RenderResult, error) {
		wouldRender := false
		didRender := false

		sandboxCfg := &sandboxConfig{
			thisBin:     thisBin,
			sandboxPath: taskDir,
			destPath:    i.Path,
			perms:       strconv.FormatUint(uint64(i.Perms), 8),
			user:        i.User,
			group:       i.Group,
			taskID:      taskID,
			contents:    i.Contents,
		}

		logs, code, err := renderTemplateInSandbox(sandboxCfg)
		if err != nil {
			if len(logs) > 0 {
				log.Printf("[ERROR] %v: %s", err, logs)
			} else {
				log.Printf("[ERROR] %v", err)
			}
			return &renderer.RenderResult{
				DidRender:   false,
				WouldRender: false,
				Contents:    []byte{},
			}, fmt.Errorf("template render subprocess failed: %w", err)
		}
		if code == trenderer.ExitWouldRenderButDidnt {
			didRender = false
			wouldRender = true
		} else {
			didRender = true
			wouldRender = true
		}

		// the subprocess emits logs matching the consul-template runner, but we
		// CT doesn't support hclog, so we just print the whole output here to
		// stderr the same way CT does so the results look seamless
		if len(logs) > 0 {
			log.Printf("[DEBUG] %s", logs)
		}

		result := &renderer.RenderResult{
			DidRender:   didRender,
			WouldRender: wouldRender,
			Contents:    i.Contents,
		}
		return result, nil
	}
}

func ReaderFn(taskID, taskDir string, sandboxEnabled bool) func(string) ([]byte, error) {
	if !sandboxEnabled {
		return nil
	}
	thisBin := subproc.Self()

	return func(src string) ([]byte, error) {

		sandboxCfg := &sandboxConfig{
			thisBin:     thisBin,
			sandboxPath: taskDir,
			sourcePath:  src,
			taskID:      taskID,
		}

		stdout, stderr, code, err := readTemplateFromSandbox(sandboxCfg)
		if err != nil && code != 0 {
			return nil, fmt.Errorf("%v: %s", err, string(stderr))
		}

		// this will get wrapped in CT log formatter
		fmt.Fprintf(os.Stderr, "[DEBUG] %s", string(stderr))
		return stdout, nil
	}
}

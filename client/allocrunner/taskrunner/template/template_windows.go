// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build windows

package template

import (
	"fmt"
	"os"

	"github.com/hashicorp/consul-template/renderer"
	"github.com/hashicorp/consul-template/signals"
)

// we don't sandbox template rendering on windows
func isSandboxEnabled(cfg *TaskTemplateManagerConfig) bool {
	return false
}

type sandboxConfig struct{}

func ReaderFn(taskID, taskDir string, sandboxEnabled bool) func(string) ([]byte, error) {
	return nil
}

func RenderFn(taskID, taskDir string, sandboxEnabled bool) func(*renderer.RenderInput) (*renderer.RenderResult, error) {
	return nil
}

func NewTaskTemplateManager(config *TaskTemplateManagerConfig) (*TaskTemplateManager, error) {
	// Check pre-conditions
	if err := config.Validate(); err != nil {
		return nil, err
	}

	tm := &TaskTemplateManager{
		config:     config,
		shutdownCh: make(chan struct{}),
	}

	// Parse the signals that we need
	for _, tmpl := range config.Templates {
		if tmpl.ChangeSignal == "" {
			continue
		}

		sig, err := signals.Parse(tmpl.ChangeSignal)
		if err != nil {
			return nil, fmt.Errorf("Failed to parse signal %q", tmpl.ChangeSignal)
		}

		if tm.signals == nil {
			tm.signals = make(map[string]os.Signal)
		}

		tm.signals[tmpl.ChangeSignal] = sig
	}

	// Build the consul-template runner
	runner, lookup, err := templateRunner(config)
	if err != nil {
		return nil, err
	}
	tm.runner = runner
	tm.lookup = lookup

	go tm.run()
	return tm, nil
}

// Stop is used to stop the consul-template runner
func (tm *TaskTemplateManager) Stop() {
	tm.shutdownLock.Lock()
	defer tm.shutdownLock.Unlock()

	if tm.shutdown {
		return
	}

	close(tm.shutdownCh)
	tm.shutdown = true

	// Stop the consul-template runner
	if tm.runner != nil {
		tm.runner.Stop()
	}
}

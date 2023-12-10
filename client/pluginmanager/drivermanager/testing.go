// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !release
// +build !release

package drivermanager

import (
	"fmt"
	"testing"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/lib/numalib"
	"github.com/hashicorp/nomad/helper/pluginutils/catalog"
	"github.com/hashicorp/nomad/helper/pluginutils/loader"
	"github.com/hashicorp/nomad/helper/pluginutils/singleton"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/drivers"
)

type testManager struct {
	logger log.Logger
	loader loader.PluginCatalog
}

var (
	topology = numalib.Scan(numalib.PlatformScanners())
)

func TestDriverManager(t *testing.T) Manager {
	logger := testlog.HCLogger(t).Named("driver_mgr")
	pluginLoader := catalog.TestPluginLoader(t)
	return &testManager{
		logger: logger,
		loader: singleton.NewSingletonLoader(logger, pluginLoader),
	}
}

func (m *testManager) Run()               {}
func (m *testManager) Shutdown()          {}
func (m *testManager) PluginType() string { return base.PluginTypeDriver }

func (m *testManager) Dispense(driver string) (drivers.DriverPlugin, error) {
	baseConfig := &base.AgentConfig{
		Driver: &base.ClientDriverConfig{
			Topology: topology,
		},
	}
	instance, err := m.loader.Dispense(driver, base.PluginTypeDriver, baseConfig, m.logger)
	if err != nil {
		return nil, err
	}

	d, ok := instance.Plugin().(drivers.DriverPlugin)
	if !ok {
		return nil, fmt.Errorf("plugin does not implement DriverPlugin interface")
	}

	return d, nil
}

func (m *testManager) RegisterEventHandler(driver, taskID string, handler EventHandler) {}
func (m *testManager) DeregisterEventHandler(driver, taskID string)                     {}

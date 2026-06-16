// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !release
// +build !release

package drivermanager

import (
	"fmt"
	"testing"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/v2/client/lib/numalib"
	"github.com/hashicorp/nomad/v2/helper/pluginutils/catalog"
	"github.com/hashicorp/nomad/v2/helper/pluginutils/loader"
	"github.com/hashicorp/nomad/v2/helper/pluginutils/singleton"
	"github.com/hashicorp/nomad/v2/helper/testlog"
	"github.com/hashicorp/nomad/v2/plugins/base"
	"github.com/hashicorp/nomad/v2/plugins/drivers"
)

type testManager struct {
	logger   log.Logger
	loader   loader.PluginCatalog
	topology *numalib.Topology
}

func TestDriverManager(t *testing.T) Manager {
	topology := numalib.Scan(numalib.PlatformScanners(false))
	logger := testlog.HCLogger(t).Named("driver_mgr")
	pluginLoader := catalog.TestPluginLoader(t)
	return &testManager{
		logger:   logger,
		loader:   singleton.NewSingletonLoader(logger, pluginLoader),
		topology: topology,
	}
}

func (m *testManager) Run()               {}
func (m *testManager) Shutdown()          {}
func (m *testManager) PluginType() string { return base.PluginTypeDriver }

func (m *testManager) Dispense(driver string) (drivers.DriverPlugin, error) {
	baseConfig := &base.AgentConfig{
		Driver: &base.ClientDriverConfig{
			Topology: m.topology,
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

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/pluginmanager/drivermanager"
	"github.com/hashicorp/nomad/helper/pluginutils/catalog"
	nconfig "github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

// TestDriverManager_Fingerprint_Run asserts that node is populated with
// driver fingerprints
func TestDriverManager_Fingerprint_Run(t *testing.T) {
	ci.Parallel(t)

	testClient, cleanup := TestClient(t, nil)
	defer cleanup()

	conf := testClient.GetConfig()
	dm := drivermanager.New(&drivermanager.Config{
		Logger:              testClient.logger,
		Loader:              conf.PluginSingletonLoader,
		PluginConfig:        conf.NomadPluginConfig(),
		Updater:             testClient.updateNodeFromDriver,
		EventHandlerFactory: testClient.GetTaskEventHandler,
		State:               testClient.stateDB,
	})

	go dm.Run()
	defer dm.Shutdown()

	testutil.WaitForResult(func() (bool, error) {
		node := testClient.Node()

		d, ok := node.Drivers["mock_driver"]
		if !ok {
			return false, fmt.Errorf("mock_driver driver is not present: %+v", node.Drivers)
		}

		if !d.Detected || !d.Healthy {
			return false, fmt.Errorf("mock_driver driver is not marked healthy: %+v", d)
		}

		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})
}

// TestDriverManager_Fingerprint_Run asserts that node is populated with
// driver fingerprints and it's updated periodically
func TestDriverManager_Fingerprint_Periodic(t *testing.T) {
	ci.Parallel(t)

	testClient, cleanup := TestClient(t, func(c *config.Config) {
		pluginConfig := []*nconfig.PluginConfig{
			{
				Name: "mock_driver",
				Config: map[string]interface{}{
					"shutdown_periodic_after":    true,
					"shutdown_periodic_duration": 2 * time.Second,
				},
			},
		}

		c.PluginLoader = catalog.TestPluginLoaderWithOptions(t, "", map[string]string{}, pluginConfig)

	})
	defer cleanup()

	conf := testClient.GetConfig()
	dm := drivermanager.New(&drivermanager.Config{
		Logger:              testClient.logger,
		Loader:              conf.PluginSingletonLoader,
		PluginConfig:        conf.NomadPluginConfig(),
		Updater:             testClient.updateNodeFromDriver,
		EventHandlerFactory: testClient.GetTaskEventHandler,
		State:               testClient.stateDB,
	})

	go dm.Run()
	defer dm.Shutdown()

	// we get a healthy mock_driver first
	testutil.WaitForResult(func() (bool, error) {
		node := testClient.Node()

		d, ok := node.Drivers["mock_driver"]
		if !ok {
			return false, fmt.Errorf("mock_driver driver is not present: %+v", node.Drivers)
		}

		if !d.Detected || !d.Healthy {
			return false, fmt.Errorf("mock_driver driver is not marked healthy: %+v", d)
		}

		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})

	// eventually, the mock_driver is marked as unhealthy
	testutil.WaitForResult(func() (bool, error) {
		node := testClient.Node()

		d, ok := node.Drivers["mock_driver"]
		if !ok {
			return false, fmt.Errorf("mock_driver driver is not present: %+v", node.Drivers)
		}

		if d.Detected || d.Healthy {
			return false, fmt.Errorf("mock_driver driver is still marked as healthy: %+v", d)
		}

		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})
}

// TestDriverManager_NodeAttributes_Run asserts that node attributes are populated
// in addition to node.Drivers until we fully deprecate it
func TestDriverManager_NodeAttributes_Run(t *testing.T) {
	ci.Parallel(t)

	testClient, cleanup := TestClient(t, func(c *config.Config) {
		c.Options = map[string]string{
			"driver.raw_exec.enable": "1",
		}
	})
	defer cleanup()

	conf := testClient.GetConfig()
	dm := drivermanager.New(&drivermanager.Config{
		Logger:              testClient.logger,
		Loader:              conf.PluginSingletonLoader,
		PluginConfig:        conf.NomadPluginConfig(),
		Updater:             testClient.updateNodeFromDriver,
		EventHandlerFactory: testClient.GetTaskEventHandler,
		State:               testClient.stateDB,
	})

	go dm.Run()
	defer dm.Shutdown()

	// we should have mock_driver as well as raw_exec in node attributes
	testutil.WaitForResult(func() (bool, error) {
		node := testClient.Node()

		// check mock driver
		if node.Attributes["driver.mock_driver"] == "" {
			return false, fmt.Errorf("mock_driver is not present in attributes: %#v", node.Attributes)
		}
		d, ok := node.Drivers["mock_driver"]
		if !ok {
			return false, fmt.Errorf("mock_driver is not present in drivers: %#v", node.Drivers)
		}

		if !d.Detected || !d.Healthy {
			return false, fmt.Errorf("mock_driver driver is not marked as healthy: %+v", d)
		}

		if d.Attributes["driver.mock_driver"] != "" {
			return false, fmt.Errorf("mock driver driver attributes contain duplicate health info: %#v", d.Attributes)
		}

		// check raw_exec
		if node.Attributes["driver.raw_exec"] == "" {
			return false, fmt.Errorf("raw_exec is not present in attributes: %#v", node.Attributes)
		}
		d, ok = node.Drivers["raw_exec"]
		if !ok {
			return false, fmt.Errorf("raw_exec is not present in drivers: %#v", node.Drivers)
		}

		if !d.Detected || !d.Healthy {
			return false, fmt.Errorf("raw_exec driver is not marked as healthy: %+v", d)
		}

		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})
}

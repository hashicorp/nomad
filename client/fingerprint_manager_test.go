package client

import (
	"fmt"
	"testing"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/shared/loader"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"

	// registering raw_exec driver plugin used in testing
	_ "github.com/hashicorp/nomad/drivers/rawexec"
)

func TestFingerprintManager_Run_MockDriver(t *testing.T) {
	t.Skip("missing mock driver plugin implementation")
	t.Parallel()
	require := require.New(t)
	testClient, cleanup := TestClient(t, nil)

	testClient.logger = testlog.HCLogger(t)
	defer cleanup()

	fm := NewFingerprintManager(
		testClient.config.PluginSingletonLoader,
		testClient.GetConfig,
		testClient.config.Node,
		testClient.shutdownCh,
		testClient.updateNodeFromFingerprint,
		testClient.updateNodeFromDriver,
		testlog.HCLogger(t),
	)

	err := fm.Run()
	require.Nil(err)

	node := testClient.config.Node

	require.NotNil(node.Drivers["mock_driver"])
	require.True(node.Drivers["mock_driver"].Detected)
	require.True(node.Drivers["mock_driver"].Healthy)
}

func TestFingerprintManager_Run_ResourcesFingerprint(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	testClient, cleanup := TestClient(t, nil)
	defer cleanup()

	fm := NewFingerprintManager(
		testClient.config.PluginSingletonLoader,
		testClient.GetConfig,
		testClient.config.Node,
		testClient.shutdownCh,
		testClient.updateNodeFromFingerprint,
		testClient.updateNodeFromDriver,
		testClient.logger,
	)

	err := fm.Run()
	require.Nil(err)

	node := testClient.config.Node

	require.NotEqual(0, node.Resources.CPU)
	require.NotEqual(0, node.Resources.MemoryMB)
	require.NotZero(node.Resources.DiskMB)
}

func TestFingerprintManager_Fingerprint_Run(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	testClient, cleanup := TestClient(t, func(c *config.Config) {
		c.Options = map[string]string{
			"driver.raw_exec.enable": "true",
		}
	})
	defer cleanup()

	fm := NewFingerprintManager(
		testClient.config.PluginSingletonLoader,
		testClient.GetConfig,
		testClient.config.Node,
		testClient.shutdownCh,
		testClient.updateNodeFromFingerprint,
		testClient.updateNodeFromDriver,
		testClient.logger,
	)

	err := fm.Run()
	require.Nil(err)

	node := testClient.config.Node

	require.NotNil(node.Drivers["raw_exec"])
	require.True(node.Drivers["raw_exec"].Detected)
	require.True(node.Drivers["raw_exec"].Healthy)
}

func TestFingerprintManager_Fingerprint_Periodic(t *testing.T) {
	t.Skip("missing mock driver plugin implementation")
	t.Parallel()
	require := require.New(t)
	testClient, cleanup := TestClient(t, func(c *config.Config) {
		c.Options = map[string]string{
			"test.shutdown_periodic_after":    "true",
			"test.shutdown_periodic_duration": "2",
		}
	})
	defer cleanup()

	fm := NewFingerprintManager(
		testClient.config.PluginSingletonLoader,
		testClient.GetConfig,
		testClient.config.Node,
		testClient.shutdownCh,
		testClient.updateNodeFromFingerprint,
		testClient.updateNodeFromDriver,
		testClient.logger,
	)

	err := fm.Run()
	require.Nil(err)

	{
		// Ensure the mock driver is registered and healthy on the client
		testutil.WaitForResult(func() (bool, error) {
			fm.nodeLock.Lock()
			defer fm.nodeLock.Unlock()
			node := fm.node
			dinfo, ok := node.Drivers["mock_driver"]
			if !ok || !dinfo.Detected || !dinfo.Healthy {
				return false, fmt.Errorf("mock driver should be detected and healthy: %+v", dinfo)
			}

			return true, nil
		}, func(err error) {
			t.Fatalf("err: %v", err)
		})
	}
	// Ensure that the client fingerprinter eventually removes this attribute and
	// marks the driver as unhealthy
	{
		testutil.WaitForResult(func() (bool, error) {
			fm.nodeLock.Lock()
			defer fm.nodeLock.Unlock()
			node := fm.node
			dinfo, ok := node.Drivers["mock_driver"]
			if !ok || dinfo.Detected || dinfo.Healthy {
				return false, fmt.Errorf("mock driver should not be detected and healthy")
			}
			return true, nil
		}, func(err error) {
			t.Fatalf("err: %v", err)
		})
	}
}

// This is a temporary measure to check that a driver has both attributes on a
// node set as well as DriverInfo.
func TestFingerprintManager_HealthCheck_Driver(t *testing.T) {
	t.Skip("missing mock driver plugin implementation")
	t.Parallel()
	require := require.New(t)
	testClient, cleanup := TestClient(t, func(c *config.Config) {
		c.Options = map[string]string{
			"driver.raw_exec.enable":          "1",
			"test.shutdown_periodic_after":    "true",
			"test.shutdown_periodic_duration": "2",
		}
	})
	defer cleanup()

	fm := NewFingerprintManager(
		testClient.config.PluginSingletonLoader,
		testClient.GetConfig,
		testClient.config.Node,
		testClient.shutdownCh,
		testClient.updateNodeFromFingerprint,
		testClient.updateNodeFromDriver,
		testClient.logger,
	)

	err := fm.Run()
	require.Nil(err)

	// Ensure the mock driver is registered and healthy on the client
	testutil.WaitForResult(func() (bool, error) {
		fm.nodeLock.Lock()
		node := fm.node
		defer fm.nodeLock.Unlock()

		mockDriverAttribute := node.Attributes["driver.mock_driver"]
		if mockDriverAttribute == "" {
			return false, fmt.Errorf("mock driver info should be set on the client attributes")
		}
		mockDriverInfo := node.Drivers["mock_driver"]
		if mockDriverInfo == nil {
			return false, fmt.Errorf("mock driver info should be set on the client")
		}
		if !mockDriverInfo.Healthy {
			return false, fmt.Errorf("mock driver info should be healthy")
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Ensure that a default driver without health checks enabled is registered and healthy on the client
	testutil.WaitForResult(func() (bool, error) {
		fm.nodeLock.Lock()
		node := fm.node
		defer fm.nodeLock.Unlock()

		rawExecAttribute := node.Attributes["driver.raw_exec"]
		if rawExecAttribute == "" {
			return false, fmt.Errorf("raw exec info should be set on the client attributes")
		}
		rawExecInfo := node.Drivers["raw_exec"]
		if rawExecInfo == nil {
			return false, fmt.Errorf("raw exec driver info should be set on the client")
		}
		if !rawExecInfo.Detected {
			return false, fmt.Errorf("raw exec driver should be detected")
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Ensure the mock driver is registered
	testutil.WaitForResult(func() (bool, error) {
		fm.nodeLock.Lock()
		node := fm.node
		defer fm.nodeLock.Unlock()

		mockDriverAttribute := node.Attributes["driver.mock_driver"]
		if mockDriverAttribute == "" {
			return false, fmt.Errorf("mock driver info should set on the client attributes")
		}
		mockDriverInfo := node.Drivers["mock_driver"]
		if mockDriverInfo == nil {
			return false, fmt.Errorf("mock driver info should be set on the client")
		}
		if !mockDriverInfo.Healthy {
			return false, fmt.Errorf("mock driver info should not be healthy")
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Ensure that we don't duplicate health check information on the driver
	// health information
	fm.nodeLock.Lock()
	node := fm.node
	fm.nodeLock.Unlock()
	mockDriverAttributes := node.Drivers["mock_driver"].Attributes
	require.NotContains(mockDriverAttributes, "driver.mock_driver")
}

func TestFingerprintManager_HealthCheck_Periodic(t *testing.T) {
	t.Skip("missing mock driver plugin implementation")
	t.Parallel()
	require := require.New(t)
	testClient, cleanup := TestClient(t, func(c *config.Config) {
		c.Options = map[string]string{
			"test.shutdown_periodic_after":    "true",
			"test.shutdown_periodic_duration": "2",
		}
	})
	defer cleanup()

	fm := NewFingerprintManager(
		testClient.config.PluginSingletonLoader,
		testClient.GetConfig,
		testClient.config.Node,
		testClient.shutdownCh,
		testClient.updateNodeFromFingerprint,
		testClient.updateNodeFromDriver,
		testClient.logger,
	)

	err := fm.Run()
	require.Nil(err)

	{
		// Ensure the mock driver is registered and healthy on the client
		testutil.WaitForResult(func() (bool, error) {
			fm.nodeLock.Lock()
			node := fm.node
			defer fm.nodeLock.Unlock()

			mockDriverInfo := node.Drivers["mock_driver"]
			if mockDriverInfo == nil {
				return false, fmt.Errorf("mock driver info should be set on the client")
			}
			if !mockDriverInfo.Detected {
				return false, fmt.Errorf("mock driver info should be detected")
			}
			if !mockDriverInfo.Healthy {
				return false, fmt.Errorf("mock driver info should be healthy")
			}
			return true, nil
		}, func(err error) {
			t.Fatalf("err: %v", err)
		})
	}
	{
		// Ensure that the client health check eventually removes this attribute and
		// marks the driver as unhealthy
		testutil.WaitForResult(func() (bool, error) {
			fm.nodeLock.Lock()
			node := fm.node
			defer fm.nodeLock.Unlock()

			mockDriverInfo := node.Drivers["mock_driver"]
			if mockDriverInfo == nil {
				return false, fmt.Errorf("mock driver info should be set on the client")
			}
			if !mockDriverInfo.Detected {
				return false, fmt.Errorf("mock driver info should be detected")
			}
			if !mockDriverInfo.Healthy {
				return false, fmt.Errorf("mock driver info should be healthy")
			}
			return true, nil
		}, func(err error) {
			t.Fatalf("err: %v", err)
		})
	}
	{
		// Ensure that the client health check eventually removes this attribute and
		// marks the driver as unhealthy
		testutil.WaitForResult(func() (bool, error) {
			fm.nodeLock.Lock()
			node := fm.node
			defer fm.nodeLock.Unlock()

			mockDriverInfo := node.Drivers["mock_driver"]
			if mockDriverInfo == nil {
				return false, fmt.Errorf("mock driver info should be set on the client")
			}
			if mockDriverInfo.Detected {
				return false, fmt.Errorf("mock driver should be detected")
			}
			if mockDriverInfo.Healthy {
				return false, fmt.Errorf("mock driver should not be healthy")
			}
			return true, nil
		}, func(err error) {
			t.Fatalf("err: %v", err)
		})
	}
}

func TestFimgerprintManager_Run_InWhitelist(t *testing.T) {
	t.Skip("missing mock driver plugin implementation")
	t.Parallel()
	require := require.New(t)

	testClient, cleanup := TestClient(t, func(c *config.Config) {
		c.Options = map[string]string{
			"test.shutdown_periodic_after":    "true",
			"test.shutdown_periodic_duration": "2",
		}
	})
	defer cleanup()

	fm := NewFingerprintManager(
		testClient.config.PluginSingletonLoader,
		testClient.GetConfig,
		testClient.config.Node,
		testClient.shutdownCh,
		testClient.updateNodeFromFingerprint,
		testClient.updateNodeFromDriver,
		testClient.logger,
	)

	err := fm.Run()
	require.Nil(err)

	node := testClient.config.Node

	require.NotEqual(node.Attributes["cpu.frequency"], "")
}

func TestFingerprintManager_Run_InBlacklist(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	testClient, cleanup := TestClient(t, func(c *config.Config) {
		c.Options = map[string]string{
			"fingerprint.whitelist": "  arch,memory,foo,bar	",
			"fingerprint.blacklist": "  cpu	",
		}
	})
	defer cleanup()

	fm := NewFingerprintManager(
		testClient.config.PluginSingletonLoader,
		testClient.GetConfig,
		testClient.config.Node,
		testClient.shutdownCh,
		testClient.updateNodeFromFingerprint,
		testClient.updateNodeFromDriver,
		testClient.logger,
	)

	err := fm.Run()
	require.Nil(err)

	node := testClient.config.Node

	require.NotContains(node.Attributes, "cpu.frequency")
	require.NotEqual(node.Attributes["memory.totalbytes"], "")
}

func TestFingerprintManager_Run_Combination(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	testClient, cleanup := TestClient(t, func(c *config.Config) {
		c.Options = map[string]string{
			"fingerprint.whitelist": "  arch,cpu,memory,foo,bar	",
			"fingerprint.blacklist": "  memory,nomad	",
		}
	})
	defer cleanup()

	fm := NewFingerprintManager(
		testClient.config.PluginSingletonLoader,
		testClient.GetConfig,
		testClient.config.Node,
		testClient.shutdownCh,
		testClient.updateNodeFromFingerprint,
		testClient.updateNodeFromDriver,
		testClient.logger,
	)

	err := fm.Run()
	require.Nil(err)

	node := testClient.config.Node

	require.NotEqual(node.Attributes["cpu.frequency"], "")
	require.NotEqual(node.Attributes["cpu.arch"], "")
	require.NotContains(node.Attributes, "memory.totalbytes")
	require.NotContains(node.Attributes, "nomad.version")
}

func TestFingerprintManager_Run_WhitelistDrivers(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	testClient, cleanup := TestClient(t, func(c *config.Config) {
		c.Options = map[string]string{
			"driver.raw_exec.enable": "1",
			"driver.whitelist": "   raw_exec ,  foo	",
		}
	})
	defer cleanup()

	fm := NewFingerprintManager(
		testClient.config.PluginSingletonLoader,
		testClient.GetConfig,
		testClient.config.Node,
		testClient.shutdownCh,
		testClient.updateNodeFromFingerprint,
		testClient.updateNodeFromDriver,
		testClient.logger,
	)

	err := fm.Run()
	require.Nil(err)

	node := testClient.config.Node
	require.NotNil(node.Drivers["raw_exec"])
	require.NotContains(node.Drivers, "java")
}

func TestFingerprintManager_Run_AllDriversBlacklisted(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	testClient, cleanup := TestClient(t, func(c *config.Config) {
		c.Options = map[string]string{
			"driver.raw_exec.enable": "1",
			"driver.whitelist": "   foo,bar,baz	",
		}
	})
	defer cleanup()

	fm := NewFingerprintManager(
		testClient.config.PluginSingletonLoader,
		testClient.GetConfig,
		testClient.config.Node,
		testClient.shutdownCh,
		testClient.updateNodeFromFingerprint,
		testClient.updateNodeFromDriver,
		testClient.logger,
	)

	err := fm.Run()
	require.Nil(err)

	node := testClient.config.Node

	require.NotContains(node.Attributes, "driver.raw_exec")
	// TODO(nickethier): uncomment after missing driver implementations added
	//require.NotContains(node.Attributes, "driver.exec")
	//require.NotContains(node.Attributes, "driver.docker")
}

func TestFingerprintManager_Run_DriversWhiteListBlacklistCombination(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	testClient, cleanup := TestClient(t, func(c *config.Config) {
		c.Options = map[string]string{
			"driver.raw_exec.enable": "1",
			"driver.whitelist": "   raw_exec,exec,foo,bar,baz	",
			"driver.blacklist": "   exec,foo,bar,baz	",
		}
	})

	testClient.logger = testlog.HCLogger(t)
	defer cleanup()

	fm := NewFingerprintManager(
		testClient.config.PluginSingletonLoader,
		testClient.GetConfig,
		testClient.config.Node,
		testClient.shutdownCh,
		testClient.updateNodeFromFingerprint,
		testClient.updateNodeFromDriver,
		testClient.logger,
	)

	err := fm.Run()
	require.Nil(err)

	node := testClient.config.Node

	require.NotNil(node.Drivers["raw_exec"])
	require.NotContains(node.Drivers, "exec")
}

func TestFingerprintManager_Run_DriverFailure(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	testClient, cleanup := TestClient(t, func(c *config.Config) {
		c.Options = map[string]string{
			"driver.raw_exec.enable": "1",
		}
	})

	testClient.logger = testlog.HCLogger(t)
	defer cleanup()

	singLoader := testClient.config.PluginSingletonLoader

	dispenseCalls := 0
	loader := &loader.MockCatalog{
		DispenseF: func(name, pluginType string, cfg *base.ClientAgentConfig, logger log.Logger) (loader.PluginInstance, error) {
			if pluginType == base.PluginTypeDriver && name == "raw_exec" {
				dispenseCalls++
			}
			return singLoader.Dispense(name, pluginType, cfg, logger)
		},
		ReattachF: singLoader.Reattach,
		CatalogF:  singLoader.Catalog,
	}

	fm := NewFingerprintManager(
		loader,
		testClient.GetConfig,
		testClient.config.Node,
		testClient.shutdownCh,
		testClient.updateNodeFromFingerprint,
		testClient.updateNodeFromDriver,
		testClient.logger,
	)

	fpChan, cancel, err := fm.dispenseDriverFingerprint("raw_exec")
	require.NoError(err)
	require.Equal(1, dispenseCalls)

	cancel()
	go fm.watchDriverFingerprint(fpChan, "raw_exec", cancel)

	testutil.WaitForResult(func() (bool, error) {
		if 2 != dispenseCalls {
			return false, fmt.Errorf("expected dispenseCalls to be 2 but was %d", dispenseCalls)
		}
		return true, nil
	}, func(err error) {
		require.NoError(err)
	})
}

func TestFingerprintManager_Run_DriversInBlacklist(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	testClient, cleanup := TestClient(t, func(c *config.Config) {
		c.Options = map[string]string{
			"driver.raw_exec.enable": "1",
			"driver.whitelist": "   raw_exec,foo,bar,baz	",
			"driver.blacklist": "   exec,foo,bar,baz	",
		}
	})

	testClient.logger = testlog.HCLogger(t)
	defer cleanup()

	fm := NewFingerprintManager(
		testClient.config.PluginSingletonLoader,
		testClient.GetConfig,
		testClient.config.Node,
		testClient.shutdownCh,
		testClient.updateNodeFromFingerprint,
		testClient.updateNodeFromDriver,
		testClient.logger,
	)

	err := fm.Run()
	require.Nil(err)

	node := testClient.config.Node

	require.NotNil(node.Drivers["raw_exec"])
	require.NotContains(node.Drivers, "exec")
}

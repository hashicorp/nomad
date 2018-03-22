package client

import (
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

func TestFingerprintManager_Run_MockDriver(t *testing.T) {
	driver.CheckForMockDriver(t)
	t.Parallel()
	require := require.New(t)

	s1, serverAddr := testServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	testClient := TestClient(t, func(c *config.Config) {
		c.RPCHandler = s1
		c.Servers = []string{serverAddr}
	})

	testClient.logger = log.New(os.Stderr, "", log.LstdFlags)
	defer testClient.Shutdown()
	waitTilNodeReady(testClient, t)

	fm := NewFingerprintManager(
		testClient.GetConfig,
		testClient.config.Node,
		testClient.shutdownCh,
		testClient.updateNodeFromFingerprint,
		testClient.updateNodeFromDriver,
		testLogger(),
	)

	err := fm.Run()
	require.Nil(err)

	node := testClient.config.Node

	require.NotEqual("", node.Attributes["driver.mock_driver"])
}

func TestFingerprintManager_Run_ResourcesFingerprint(t *testing.T) {
	driver.CheckForMockDriver(t)
	t.Parallel()
	require := require.New(t)

	s1, serverAddr := testServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	testClient := TestClient(t, func(c *config.Config) {
		c.RPCHandler = s1
		c.Servers = []string{serverAddr}
	})

	testClient.logger = testLogger()
	defer testClient.Shutdown()
	waitTilNodeReady(testClient, t)

	fm := NewFingerprintManager(
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

	s1, serverAddr := testServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	testClient := TestClient(t, func(c *config.Config) {
		c.RPCHandler = s1
		c.Servers = []string{serverAddr}
	})

	testClient.logger = testLogger()
	defer testClient.Shutdown()
	waitTilNodeReady(testClient, t)

	fm := NewFingerprintManager(
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

	require.NotEqual("", node.Attributes["driver.raw_exec"])
	require.True(node.Drivers["raw_exec"].Detected)
	require.True(node.Drivers["raw_exec"].Healthy)
}

func TestFingerprintManager_Fingerprint_Periodic(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	s1, serverAddr := testServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	testClient := TestClient(t, func(c *config.Config) {
		c.RPCHandler = s1
		c.Servers = []string{serverAddr}
		c.Options = map[string]string{
			"driver.raw_exec.enable":          "1",
			"test.shutdown_periodic_after":    "true",
			"test.shutdown_periodic_duration": "2",
		}
	})

	testClient.logger = testLogger()
	defer testClient.Shutdown()
	waitTilNodeReady(testClient, t)

	fm := NewFingerprintManager(
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

			mockDriverStatus := node.Attributes["driver.mock_driver"]
			if mockDriverStatus == "" {
				return false, fmt.Errorf("mock driver attribute should be set on the client")
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
			node := fm.node
			defer fm.nodeLock.Unlock()

			mockDriverStatus := node.Attributes["driver.mock_driver"]
			if mockDriverStatus != "" {
				return false, fmt.Errorf("mock driver attribute should not be set on the client")
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
	t.Parallel()
	require := require.New(t)

	s1, serverAddr := testServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	testClient := TestClient(t, func(c *config.Config) {
		c.RPCHandler = s1
		c.Servers = []string{serverAddr}
		c.Options = map[string]string{
			"driver.raw_exec.enable":          "1",
			"test.shutdown_periodic_after":    "true",
			"test.shutdown_periodic_duration": "2",
		}
	})

	testClient.logger = testLogger()
	defer testClient.Shutdown()
	waitTilNodeReady(testClient, t)

	fm := NewFingerprintManager(
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

	// Ensure the mock driver is de-registered when it becomes unhealthy
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
			return false, fmt.Errorf("mock driver info should not be healthy")
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestFingerprintManager_HealthCheck_Periodic(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	s1, serverAddr := testServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	testClient := TestClient(t, func(c *config.Config) {
		c.RPCHandler = s1
		c.Servers = []string{serverAddr}
		c.Options = map[string]string{
			"driver.raw_exec.enable":          "1",
			"test.shutdown_periodic_after":    "true",
			"test.shutdown_periodic_duration": "2",
		}
	})

	testClient.logger = testLogger()
	defer testClient.Shutdown()
	waitTilNodeReady(testClient, t)

	fm := NewFingerprintManager(
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
	t.Parallel()
	require := require.New(t)

	s1, serverAddr := testServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	testClient := TestClient(t, func(c *config.Config) {
		c.RPCHandler = s1
		c.Servers = []string{serverAddr}
		c.Options = map[string]string{
			"driver.raw_exec.enable":          "1",
			"test.shutdown_periodic_after":    "true",
			"test.shutdown_periodic_duration": "2",
		}
	})

	testClient.logger = testLogger()
	defer testClient.Shutdown()
	waitTilNodeReady(testClient, t)

	fm := NewFingerprintManager(
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

	s1, serverAddr := testServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	testClient := TestClient(t, func(c *config.Config) {
		c.RPCHandler = s1
		c.Servers = []string{serverAddr}
		c.Options = map[string]string{
			"fingerprint.whitelist": "  arch,memory,foo,bar	",
			"fingerprint.blacklist": "  cpu	",
		}
	})

	testClient.logger = testLogger()
	defer testClient.Shutdown()
	waitTilNodeReady(testClient, t)

	fm := NewFingerprintManager(
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

	require.Equal(node.Attributes["cpu.frequency"], "")
	require.NotEqual(node.Attributes["memory.totalbytes"], "")
}

func TestFingerprintManager_Run_Combination(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	s1, serverAddr := testServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	testClient := TestClient(t, func(c *config.Config) {
		c.RPCHandler = s1
		c.Servers = []string{serverAddr}
		c.Options = map[string]string{
			"fingerprint.whitelist": "  arch,cpu,memory,foo,bar	",
			"fingerprint.blacklist": "  memory,nomad	",
		}
	})

	testClient.logger = testLogger()
	defer testClient.Shutdown()
	waitTilNodeReady(testClient, t)

	fm := NewFingerprintManager(
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
	require.Equal(node.Attributes["memory.totalbytes"], "")
	require.Equal(node.Attributes["nomad.version"], "")
}

func TestFingerprintManager_Run_WhitelistDrivers(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	s1, serverAddr := testServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	testClient := TestClient(t, func(c *config.Config) {
		c.RPCHandler = s1
		c.Servers = []string{serverAddr}
		c.Options = map[string]string{
			"driver.raw_exec.enable": "1",
			"driver.whitelist": "   raw_exec ,  foo	",
		}
	})

	testClient.logger = testLogger()
	defer testClient.Shutdown()
	waitTilNodeReady(testClient, t)

	fm := NewFingerprintManager(
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
	require.NotEqual(node.Attributes["driver.raw_exec"], "")
}

func TestFingerprintManager_Run_AllDriversBlacklisted(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	s1, serverAddr := testServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	testClient := TestClient(t, func(c *config.Config) {
		c.RPCHandler = s1
		c.Servers = []string{serverAddr}
		c.Options = map[string]string{
			"driver.whitelist": "   foo,bar,baz	",
		}
	})

	testClient.logger = testLogger()
	defer testClient.Shutdown()
	waitTilNodeReady(testClient, t)

	fm := NewFingerprintManager(
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

	require.Equal(node.Attributes["driver.raw_exec"], "")
	require.Equal(node.Attributes["driver.exec"], "")
	require.Equal(node.Attributes["driver.docker"], "")
}

func TestFingerprintManager_Run_DriversWhiteListBlacklistCombination(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	s1, serverAddr := testServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	testClient := TestClient(t, func(c *config.Config) {
		c.RPCHandler = s1
		c.Servers = []string{serverAddr}
		c.Options = map[string]string{
			"driver.raw_exec.enable": "1",
			"driver.whitelist": "   raw_exec,exec,foo,bar,baz	",
			"driver.blacklist": "   exec,foo,bar,baz	",
		}
	})

	testClient.logger = testLogger()
	defer testClient.Shutdown()
	waitTilNodeReady(testClient, t)

	fm := NewFingerprintManager(
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

	require.NotEqual(node.Attributes["driver.raw_exec"], "")
	require.Equal(node.Attributes["driver.exec"], "")
	require.Equal(node.Attributes["foo"], "")
	require.Equal(node.Attributes["bar"], "")
	require.Equal(node.Attributes["baz"], "")
}

func TestFingerprintManager_Run_DriversInBlacklist(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	s1, serverAddr := testServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	testClient := TestClient(t, func(c *config.Config) {
		c.RPCHandler = s1
		c.Servers = []string{serverAddr}
		c.Options = map[string]string{
			"driver.raw_exec.enable": "1",
			"driver.whitelist": "   raw_exec,foo,bar,baz	",
			"driver.blacklist": "   exec,foo,bar,baz	",
		}
	})

	testClient.logger = testLogger()
	defer testClient.Shutdown()
	waitTilNodeReady(testClient, t)

	fm := NewFingerprintManager(
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

	require.NotEqual(node.Attributes["driver.raw_exec"], "")
	require.Equal(node.Attributes["driver.exec"], "")
}

package client

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

func TestFingerprintManager_Run_MockDriver(t *testing.T) {
	driver.CheckForMockDriver(t)
	t.Parallel()
	require := require.New(t)

	node := &structs.Node{
		Attributes: make(map[string]string, 0),
		Links:      make(map[string]string, 0),
		Resources:  &structs.Resources{},
		Drivers:    make(map[string]*structs.DriverInfo, 0),
	}
	testConfig := config.Config{Node: node}
	testClient := &Client{config: &testConfig}
	conf := config.DefaultConfig()

	getConfig := func() *config.Config {
		return conf
	}

	fm := NewFingerprintManager(
		getConfig,
		node,
		make(chan struct{}),
		testClient.updateNodeFromFingerprint,
		testClient.updateNodeFromHealthCheck,
		testLogger(),
	)

	err := fm.Run()
	require.Nil(err)
	require.NotEqual("", node.Attributes["driver.mock_driver"])
}

func TestFingerprintManager_Run_ResourcesFingerprint(t *testing.T) {
	driver.CheckForMockDriver(t)
	t.Parallel()
	require := require.New(t)

	node := &structs.Node{
		Attributes: make(map[string]string, 0),
		Links:      make(map[string]string, 0),
		Resources:  &structs.Resources{},
		Drivers:    make(map[string]*structs.DriverInfo, 0),
	}
	testConfig := config.Config{Node: node}
	testClient := &Client{config: &testConfig}

	conf := config.DefaultConfig()
	getConfig := func() *config.Config {
		return conf
	}

	fm := NewFingerprintManager(
		getConfig,
		node,
		make(chan struct{}),
		testClient.updateNodeFromFingerprint,
		testClient.updateNodeFromHealthCheck,
		testLogger(),
	)

	err := fm.Run()
	require.Nil(err)
	require.NotEqual(0, node.Resources.CPU)
	require.NotEqual(0, node.Resources.MemoryMB)
	require.NotZero(node.Resources.DiskMB)
}

func TestFingerprintManager_Fingerprint_Run(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	node := &structs.Node{
		Attributes: make(map[string]string, 0),
		Links:      make(map[string]string, 0),
		Resources:  &structs.Resources{},
		Drivers:    make(map[string]*structs.DriverInfo, 0),
	}
	testConfig := config.Config{Node: node}
	testClient := &Client{config: &testConfig}

	conf := config.DefaultConfig()
	conf.Options = map[string]string{"driver.raw_exec.enable": "true"}
	getConfig := func() *config.Config {
		return conf
	}

	fm := NewFingerprintManager(
		getConfig,
		node,
		make(chan struct{}),
		testClient.updateNodeFromFingerprint,
		testClient.updateNodeFromHealthCheck,
		testLogger(),
	)

	err := fm.Run()
	require.Nil(err)

	require.NotEqual("", node.Attributes["driver.raw_exec"])
	require.True(node.Drivers["raw_exec"].Detected)
	require.True(node.Drivers["raw_exec"].Healthy)
}

func TestFingerprintManager_Fingerprint_Periodic(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	node := &structs.Node{
		Attributes: make(map[string]string, 0),
		Links:      make(map[string]string, 0),
		Resources:  &structs.Resources{},
		Drivers:    make(map[string]*structs.DriverInfo, 0),
	}
	testConfig := config.Config{Node: node}
	testClient := &Client{config: &testConfig}

	conf := config.DefaultConfig()
	conf.Options = map[string]string{
		"test.shutdown_periodic_after":    "true",
		"test.shutdown_periodic_duration": "3",
	}
	getConfig := func() *config.Config {
		return conf
	}

	shutdownCh := make(chan struct{})
	defer (func() {
		close(shutdownCh)
	})()

	fm := NewFingerprintManager(
		getConfig,
		node,
		shutdownCh,
		testClient.updateNodeFromFingerprint,
		testClient.updateNodeFromHealthCheck,
		testLogger(),
	)

	err := fm.Run()
	require.Nil(err)

	{
		// Ensure the mock driver is registered and healthy on the client
		testutil.WaitForResult(func() (bool, error) {
			fm.nodeLock.Lock()
			mockDriverStatus := node.Attributes["driver.mock_driver"]
			fm.nodeLock.Unlock()
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
			mockDriverStatus := node.Attributes["driver.mock_driver"]
			fm.nodeLock.Unlock()
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

	node := &structs.Node{
		Attributes: make(map[string]string, 0),
		Drivers:    make(map[string]*structs.DriverInfo, 0),
	}
	updateNode := func(r *cstructs.FingerprintResponse) *structs.Node {
		if r.Attributes != nil {
			for k, v := range r.Attributes {
				node.Attributes[k] = v
			}
		}
		return node
	}
	updateHealthCheck := func(resp *cstructs.HealthCheckResponse) *structs.Node {
		if resp.Drivers != nil {
			for k, v := range resp.Drivers {
				node.Drivers[k] = v
			}
		}
		return node
	}
	conf := config.DefaultConfig()
	conf.Options = map[string]string{
		"driver.raw_exec.enable":          "1",
		"test.shutdown_periodic_after":    "true",
		"test.shutdown_periodic_duration": "2",
	}
	getConfig := func() *config.Config {
		return conf
	}

	shutdownCh := make(chan struct{})
	defer (func() {
		close(shutdownCh)
	})()

	fm := NewFingerprintManager(
		getConfig,
		node,
		shutdownCh,
		updateNode,
		updateHealthCheck,
		testLogger(),
	)

	err := fm.Run()
	require.Nil(err)

	// Ensure the mock driver is registered and healthy on the client
	testutil.WaitForResult(func() (bool, error) {
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

	node := &structs.Node{
		Attributes: make(map[string]string, 0),
		Links:      make(map[string]string, 0),
		Resources:  &structs.Resources{},
		Drivers:    make(map[string]*structs.DriverInfo, 0),
	}
	testConfig := config.Config{Node: node}
	testClient := &Client{config: &testConfig}

	conf := config.DefaultConfig()
	conf.Options = map[string]string{
		"test.shutdown_periodic_after":    "true",
		"test.shutdown_periodic_duration": "3",
	}
	getConfig := func() *config.Config {
		return conf
	}

	shutdownCh := make(chan struct{})
	defer (func() {
		close(shutdownCh)
	})()

	fm := NewFingerprintManager(
		getConfig,
		node,
		shutdownCh,
		testClient.updateNodeFromFingerprint,
		testClient.updateNodeFromHealthCheck,
		testLogger(),
	)

	err := fm.Run()
	require.Nil(err)

	{
		// Ensure the mock driver is registered and healthy on the client
		testutil.WaitForResult(func() (bool, error) {
			fm.nodeLock.Lock()
			mockDriverInfo := node.Drivers["mock_driver"]
			fm.nodeLock.Unlock()
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
			mockDriverInfo := node.Drivers["mock_driver"]
			fm.nodeLock.Unlock()
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
			mockDriverInfo := node.Drivers["mock_driver"]
			fm.nodeLock.Unlock()
			if mockDriverInfo == nil {
				return false, fmt.Errorf("mock driver info should be set on the client")
			}
			if !mockDriverInfo.Detected {
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

	node := &structs.Node{
		Attributes: make(map[string]string, 0),
		Links:      make(map[string]string, 0),
		Resources:  &structs.Resources{},
		Drivers:    make(map[string]*structs.DriverInfo, 0),
	}
	testConfig := config.Config{Node: node}
	testClient := &Client{config: &testConfig}

	conf := config.DefaultConfig()
	conf.Options = map[string]string{"fingerprint.whitelist": "  arch,cpu,memory,network,storage,foo,bar	"}
	getConfig := func() *config.Config {
		return conf
	}

	shutdownCh := make(chan struct{})
	defer (func() {
		close(shutdownCh)
	})()

	fm := NewFingerprintManager(
		getConfig,
		node,
		shutdownCh,
		testClient.updateNodeFromFingerprint,
		testClient.updateNodeFromHealthCheck,
		testLogger(),
	)

	err := fm.Run()
	require.Nil(err)
	require.NotEqual(node.Attributes["cpu.frequency"], "")
}

func TestFingerprintManager_Run_InBlacklist(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	node := &structs.Node{
		Attributes: make(map[string]string, 0),
		Links:      make(map[string]string, 0),
		Resources:  &structs.Resources{},
		Drivers:    make(map[string]*structs.DriverInfo, 0),
	}
	testConfig := config.Config{Node: node}
	testClient := &Client{config: &testConfig}

	conf := config.DefaultConfig()
	conf.Options = map[string]string{"fingerprint.whitelist": "  arch,memory,foo,bar	"}
	conf.Options = map[string]string{"fingerprint.blacklist": "  cpu	"}
	getConfig := func() *config.Config {
		return conf
	}

	shutdownCh := make(chan struct{})
	defer (func() {
		close(shutdownCh)
	})()

	fm := NewFingerprintManager(
		getConfig,
		node,
		shutdownCh,
		testClient.updateNodeFromFingerprint,
		testClient.updateNodeFromHealthCheck,
		testLogger(),
	)

	err := fm.Run()
	require.Nil(err)
	require.Equal(node.Attributes["cpu.frequency"], "")
	require.NotEqual(node.Attributes["memory.totalbytes"], "")
}

func TestFingerprintManager_Run_Combination(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	node := &structs.Node{
		Attributes: make(map[string]string, 0),
		Links:      make(map[string]string, 0),
		Resources:  &structs.Resources{},
		Drivers:    make(map[string]*structs.DriverInfo, 0),
	}
	testConfig := config.Config{Node: node}
	testClient := &Client{config: &testConfig}

	conf := config.DefaultConfig()
	conf.Options = map[string]string{"fingerprint.whitelist": "  arch,cpu,memory,foo,bar	"}
	conf.Options = map[string]string{"fingerprint.blacklist": "  memory,nomad	"}
	getConfig := func() *config.Config {
		return conf
	}

	shutdownCh := make(chan struct{})
	defer (func() {
		close(shutdownCh)
	})()

	fm := NewFingerprintManager(
		getConfig,
		node,
		shutdownCh,
		testClient.updateNodeFromFingerprint,
		testClient.updateNodeFromHealthCheck,
		testLogger(),
	)

	err := fm.Run()
	require.Nil(err)
	require.NotEqual(node.Attributes["cpu.frequency"], "")
	require.NotEqual(node.Attributes["cpu.arch"], "")
	require.Equal(node.Attributes["memory.totalbytes"], "")
	require.Equal(node.Attributes["nomad.version"], "")
}

func TestFingerprintManager_Run_WhitelistDrivers(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	node := &structs.Node{
		Attributes: make(map[string]string, 0),
		Links:      make(map[string]string, 0),
		Resources:  &structs.Resources{},
		Drivers:    make(map[string]*structs.DriverInfo, 0),
	}
	testConfig := config.Config{Node: node}
	testClient := &Client{config: &testConfig}

	conf := config.DefaultConfig()
	conf.Options = map[string]string{
		"driver.raw_exec.enable": "1",
		"driver.whitelist": "   raw_exec ,  foo	",
	}
	getConfig := func() *config.Config {
		return conf
	}

	shutdownCh := make(chan struct{})
	defer (func() {
		close(shutdownCh)
	})()

	fm := NewFingerprintManager(
		getConfig,
		node,
		shutdownCh,
		testClient.updateNodeFromFingerprint,
		testClient.updateNodeFromHealthCheck,
		testLogger(),
	)

	err := fm.Run()
	require.Nil(err)
	require.NotEqual(node.Attributes["driver.raw_exec"], "")
}

func TestFingerprintManager_Run_AllDriversBlacklisted(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	node := &structs.Node{
		Attributes: make(map[string]string, 0),
		Links:      make(map[string]string, 0),
		Resources:  &structs.Resources{},
		Drivers:    make(map[string]*structs.DriverInfo, 0),
	}
	testConfig := config.Config{Node: node}
	testClient := &Client{config: &testConfig}

	conf := config.DefaultConfig()
	conf.Options = map[string]string{
		"driver.whitelist": "   foo,bar,baz	",
	}
	getConfig := func() *config.Config {
		return conf
	}

	shutdownCh := make(chan struct{})
	defer (func() {
		close(shutdownCh)
	})()

	fm := NewFingerprintManager(
		getConfig,
		node,
		shutdownCh,
		testClient.updateNodeFromFingerprint,
		testClient.updateNodeFromHealthCheck,
		testLogger(),
	)

	err := fm.Run()
	require.Nil(err)
	require.Equal(node.Attributes["driver.raw_exec"], "")
	require.Equal(node.Attributes["driver.exec"], "")
	require.Equal(node.Attributes["driver.docker"], "")
}

func TestFingerprintManager_Run_DriversWhiteListBlacklistCombination(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	node := &structs.Node{
		Attributes: make(map[string]string, 0),
		Links:      make(map[string]string, 0),
		Resources:  &structs.Resources{},
		Drivers:    make(map[string]*structs.DriverInfo, 0),
	}
	testConfig := config.Config{Node: node}
	testClient := &Client{config: &testConfig}

	conf := config.DefaultConfig()
	conf.Options = map[string]string{
		"driver.raw_exec.enable": "1",
		"driver.whitelist": "   raw_exec,exec,foo,bar,baz	",
		"driver.blacklist": "   exec,foo,bar,baz	",
	}
	getConfig := func() *config.Config {
		return conf
	}

	shutdownCh := make(chan struct{})
	defer (func() {
		close(shutdownCh)
	})()

	fm := NewFingerprintManager(
		getConfig,
		node,
		shutdownCh,
		testClient.updateNodeFromFingerprint,
		testClient.updateNodeFromHealthCheck,
		testLogger(),
	)

	err := fm.Run()
	require.Nil(err)
	require.NotEqual(node.Attributes["driver.raw_exec"], "")
	require.Equal(node.Attributes["driver.exec"], "")
	require.Equal(node.Attributes["foo"], "")
	require.Equal(node.Attributes["bar"], "")
	require.Equal(node.Attributes["baz"], "")
}

func TestFingerprintManager_Run_DriversInBlacklist(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	node := &structs.Node{
		Attributes: make(map[string]string, 0),
		Links:      make(map[string]string, 0),
		Resources:  &structs.Resources{},
		Drivers:    make(map[string]*structs.DriverInfo, 0),
	}
	conf := config.DefaultConfig()
	conf.Options = map[string]string{
		"driver.raw_exec.enable": "1",
		"driver.whitelist": "   raw_exec,foo,bar,baz	",
		"driver.blacklist": "   exec,foo,bar,baz	",
	}
	conf.Node = node

	testClient := &Client{config: conf}

	shutdownCh := make(chan struct{})
	defer (func() {
		close(shutdownCh)
	})()

	fm := NewFingerprintManager(
		testClient.GetConfig,
		node,
		shutdownCh,
		testClient.updateNodeFromFingerprint,
		testClient.updateNodeFromHealthCheck,
		testLogger(),
	)

	err := fm.Run()
	require.Nil(err)
	require.NotEqual(node.Attributes["driver.raw_exec"], "")
	require.Equal(node.Attributes["driver.exec"], "")
}

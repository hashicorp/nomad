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
	}

	updateNode := func(r *cstructs.FingerprintResponse) *structs.Node {
		for k, v := range r.Attributes {
			node.Attributes[k] = v
		}
		return node
	}
	conf := config.DefaultConfig()
	getConfig := func() *config.Config {
		return conf
	}

	fm := NewFingerprintManager(
		getConfig,
		node,
		make(chan struct{}),
		updateNode,
		testLogger(),
	)

	err := fm.Run()
	require.Nil(err)
	require.NotEqual("", node.Attributes["driver.mock_driver"])
}

func TestFingerprintManager_Fingerprint_Run(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	node := &structs.Node{
		Attributes: make(map[string]string, 0),
	}

	conf := config.DefaultConfig()
	conf.Options = map[string]string{"driver.raw_exec.enable": "true"}
	getConfig := func() *config.Config {
		return conf
	}

	updateNode := func(r *cstructs.FingerprintResponse) *structs.Node {
		for k, v := range r.Attributes {
			node.Attributes[k] = v
		}
		return node
	}

	fm := NewFingerprintManager(
		getConfig,
		node,
		make(chan struct{}),
		updateNode,
		testLogger(),
	)

	err := fm.Run()
	require.Nil(err)

	require.NotEqual("", node.Attributes["driver.raw_exec"])
}

func TestFingerprintManager_Fingerprint_Periodic(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	node := &structs.Node{
		Attributes: make(map[string]string, 0),
	}
	updateNode := func(r *cstructs.FingerprintResponse) *structs.Node {
		for k, v := range r.Attributes {
			node.Attributes[k] = v
		}
		return node
	}
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
		updateNode,
		testLogger(),
	)

	err := fm.Run()
	require.Nil(err)

	// Ensure the mock driver is registered on the client
	testutil.WaitForResult(func() (bool, error) {
		mockDriverStatus := node.Attributes["driver.mock_driver"]
		if mockDriverStatus == "" {
			return false, fmt.Errorf("mock driver attribute should be set on the client")
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Ensure that the client fingerprinter eventually removes this attribute
	testutil.WaitForResult(func() (bool, error) {
		mockDriverStatus := node.Attributes["driver.mock_driver"]
		if mockDriverStatus != "" {
			return false, fmt.Errorf("mock driver attribute should not be set on the client")
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestFimgerprintManager_Run_InWhitelist(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	node := &structs.Node{
		Attributes: make(map[string]string, 0),
	}
	updateNode := func(r *cstructs.FingerprintResponse) *structs.Node {
		for k, v := range r.Attributes {
			node.Attributes[k] = v
		}
		return node
	}
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
		updateNode,
		testLogger(),
	)

	err := fm.Run()
	require.Nil(err)
	require.NotEqual(node.Attributes["cpu.frequency"], "")
}

func TestFimgerprintManager_Run_InBlacklist(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	node := &structs.Node{
		Attributes: make(map[string]string, 0),
	}
	updateNode := func(r *cstructs.FingerprintResponse) *structs.Node {
		for k, v := range r.Attributes {
			node.Attributes[k] = v
		}
		return node
	}
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
		updateNode,
		testLogger(),
	)

	err := fm.Run()
	require.Nil(err)
	require.Equal(node.Attributes["cpu.frequency"], "")
	require.NotEqual(node.Attributes["memory.totalbytes"], "")
}

func TestFimgerprintManager_Run_Combination(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	node := &structs.Node{
		Attributes: make(map[string]string, 0),
	}
	updateNode := func(r *cstructs.FingerprintResponse) *structs.Node {
		for k, v := range r.Attributes {
			node.Attributes[k] = v
		}
		return node
	}
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
		updateNode,
		testLogger(),
	)

	err := fm.Run()
	require.Nil(err)
	require.NotEqual(node.Attributes["cpu.frequency"], "")
	require.NotEqual(node.Attributes["cpu.arch"], "")
	require.Equal(node.Attributes["memory.totalbytes"], "")
	require.Equal(node.Attributes["nomad.version"], "")
}

func TestFimgerprintManager_Run_WhitelistDrivers(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	node := &structs.Node{
		Attributes: make(map[string]string, 0),
	}
	updateNode := func(r *cstructs.FingerprintResponse) *structs.Node {
		for k, v := range r.Attributes {
			node.Attributes[k] = v
		}
		return node
	}
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
		updateNode,
		testLogger(),
	)

	err := fm.Run()
	require.Nil(err)
	require.NotEqual(node.Attributes["driver.raw_exec"], "")
}

func TestFimgerprintManager_Run_AllDriversBlacklisted(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	node := &structs.Node{
		Attributes: make(map[string]string, 0),
	}
	updateNode := func(r *cstructs.FingerprintResponse) *structs.Node {
		for k, v := range r.Attributes {
			node.Attributes[k] = v
		}
		return node
	}
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
		updateNode,
		testLogger(),
	)

	err := fm.Run()
	require.Nil(err)
	require.Equal(node.Attributes["driver.raw_exec"], "")
	require.Equal(node.Attributes["driver.exec"], "")
	require.Equal(node.Attributes["driver.docker"], "")
}

func TestFimgerprintManager_Run_DriversWhiteListBlacklistCombination(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	node := &structs.Node{
		Attributes: make(map[string]string, 0),
	}
	updateNode := func(r *cstructs.FingerprintResponse) *structs.Node {
		for k, v := range r.Attributes {
			node.Attributes[k] = v
		}
		return node
	}
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
		updateNode,
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

func TestFimgerprintManager_Run_DriversInBlacklist(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	node := &structs.Node{
		Attributes: make(map[string]string, 0),
	}
	updateNode := func(r *cstructs.FingerprintResponse) *structs.Node {
		for k, v := range r.Attributes {
			node.Attributes[k] = v
		}
		return node
	}
	conf := config.DefaultConfig()
	conf.Options = map[string]string{
		"driver.raw_exec.enable": "1",
		"driver.whitelist": "   raw_exec,foo,bar,baz	",
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
		updateNode,
		testLogger(),
	)

	err := fm.Run()
	require.Nil(err)
	require.NotEqual(node.Attributes["driver.raw_exec"], "")
	require.Equal(node.Attributes["driver.exec"], "")
}

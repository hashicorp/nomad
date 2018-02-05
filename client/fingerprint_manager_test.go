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

// test that the driver manager updates a node when its attributes change
func TestFingerprintManager_Fingerprint_MockDriver(t *testing.T) {
	if _, ok := driver.BuiltinDrivers["mock_driver"]; !ok {
		t.Skip(`test requires mock_driver; run with "-tags nomad_test"`)
	}
	t.Parallel()
	require := require.New(t)

	node := &structs.Node{
		Attributes: make(map[string]string, 0),
	}
	mockConfig := &config.Config{
		Node: node,
	}

	var resp *cstructs.FingerprintResponse
	updateNode := func(r *cstructs.FingerprintResponse) {
		resp = r
	}

	getConfig := func() *config.Config {
		return mockConfig
	}

	fm := FingerprintManager{
		getConfig:  getConfig,
		node:       node,
		updateNode: updateNode,
		logger:     testLogger(),
	}

	// test setting up a mock driver
	drivers := []string{"mock_driver"}
	err := fm.SetupDrivers(drivers)
	require.Nil(err)

	require.NotEqual("", resp.Attributes["driver.mock_driver"])
}

func TestFingerprintManager_Fingerprint_RawExec(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	node := &structs.Node{
		Attributes: make(map[string]string, 0),
	}
	mockConfig := &config.Config{
		Node: node,
		Options: map[string]string{
			"driver.raw_exec.enable": "true",
		},
	}
	var resp *cstructs.FingerprintResponse
	updateNode := func(r *cstructs.FingerprintResponse) {
		resp = r
	}

	getConfig := func() *config.Config {
		return mockConfig
	}

	fm := FingerprintManager{
		getConfig:  getConfig,
		node:       node,
		updateNode: updateNode,
		logger:     testLogger(),
	}

	// test setting up a mock driver
	drivers := []string{"raw_exec"}
	err := fm.SetupDrivers(drivers)
	require.Nil(err)

	require.NotEqual("", resp.Attributes["driver.raw_exec"])
}

func TestFingerprintManager_Fingerprint_Periodic(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	node := &structs.Node{
		Attributes: make(map[string]string, 0),
	}
	var resp *cstructs.FingerprintResponse
	updateNode := func(r *cstructs.FingerprintResponse) {
		resp = r
	}
	mockConfig := &config.Config{
		Node: node,
		Options: map[string]string{
			"test.shutdown_periodic_after":    "true",
			"test.shutdown_periodic_duration": "3",
		},
	}

	getConfig := func() *config.Config {
		return mockConfig
	}

	shutdownCh := make(chan struct{})
	defer (func() {
		close(shutdownCh)
	})()

	fm := FingerprintManager{
		getConfig:  getConfig,
		node:       node,
		updateNode: updateNode,
		shutdownCh: shutdownCh,
		logger:     testLogger(),
	}

	// test setting up a mock driver
	drivers := []string{"mock_driver"}
	err := fm.SetupDrivers(drivers)
	require.Nil(err)

	// Ensure the mock driver is registered on the client
	testutil.WaitForResult(func() (bool, error) {
		mockDriverStatus := resp.Attributes["driver.mock_driver"]
		if mockDriverStatus == "" {
			return false, fmt.Errorf("mock driver attribute should be set on the client")
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Ensure that the client fingerprinter eventually removes this attribute
	testutil.WaitForResult(func() (bool, error) {
		mockDriverStatus := resp.Attributes["driver.mock_driver"]
		if mockDriverStatus != "" {
			return false, fmt.Errorf("mock driver attribute should not be set on the client")
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

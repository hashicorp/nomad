package client

import (
	"testing"

	"github.com/hashicorp/nomad/client/config"
	"github.com/stretchr/testify/require"
)

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
		testClient.logger,
	)

	err := fm.Run()
	require.Nil(err)

	node := testClient.config.Node

	require.NotEqual(0, node.Resources.CPU)
	require.NotEqual(0, node.Resources.MemoryMB)
	require.NotZero(node.Resources.DiskMB)
}

func TestFimgerprintManager_Run_InWhitelist(t *testing.T) {
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
			"fingerprint.blacklist": "  memory,host	",
		}
	})
	defer cleanup()

	fm := NewFingerprintManager(
		testClient.config.PluginSingletonLoader,
		testClient.GetConfig,
		testClient.config.Node,
		testClient.shutdownCh,
		testClient.updateNodeFromFingerprint,
		testClient.logger,
	)

	err := fm.Run()
	require.Nil(err)

	node := testClient.config.Node

	require.NotEqual(node.Attributes["cpu.frequency"], "")
	require.NotEqual(node.Attributes["cpu.arch"], "")
	require.NotContains(node.Attributes, "memory.totalbytes")
	require.NotContains(node.Attributes, "os.name")
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/config"
	"github.com/shoenig/test/must"
)

func TestFingerprintManager_Run_ResourcesFingerprint(t *testing.T) {
	ci.Parallel(t)

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

	_, err := fm.Run()
	must.NoError(t, err)

	node := testClient.config.Node

	must.Positive(t, node.Resources.CPU)
	must.Positive(t, node.Resources.MemoryMB)
	must.Positive(t, node.Resources.DiskMB)
}

func TestFimgerprintManager_Run_InWhitelist(t *testing.T) {
	ci.Parallel(t)

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

	_, err := fm.Run()
	must.NoError(t, err)

	node := testClient.config.Node
	must.NotEq(t, "", node.Attributes["cpu.numcores"])
}

func TestFingerprintManager_Run_InDenylist(t *testing.T) {
	ci.Parallel(t)

	testClient, cleanup := TestClient(t, func(c *config.Config) {
		c.Options = map[string]string{
			"fingerprint.allowlist": "  arch,memory,foo,bar	",
			"fingerprint.denylist":  "  cpu	",
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

	_, err := fm.Run()
	must.NoError(t, err)

	node := testClient.config.Node

	must.MapNotContainsKey(t, node.Attributes, "cpu.frequency")
	must.NotEq(t, node.Attributes["memory.totalbytes"], "")
}

func TestFingerprintManager_Run_Combination(t *testing.T) {
	ci.Parallel(t)

	testClient, cleanup := TestClient(t, func(c *config.Config) {
		c.Options = map[string]string{
			"fingerprint.allowlist": "  arch,cpu,memory,foo,bar	",
			"fingerprint.denylist":  "  memory,host	",
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

	_, err := fm.Run()
	must.NoError(t, err)

	node := testClient.config.Node

	must.NotEq(t, "", node.Attributes["cpu.numcores"])
	must.NotEq(t, "", node.Attributes["cpu.arch"])
	must.MapNotContainsKey(t, node.Attributes, "memory.totalbytes")
	must.MapNotContainsKey(t, node.Attributes, "os.name")
}

func TestFingerprintManager_Run_CombinationLegacyNames(t *testing.T) {
	ci.Parallel(t)

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

	_, err := fm.Run()
	must.NoError(t, err)

	node := testClient.config.Node
	must.NotEq(t, "", node.Attributes["cpu.numcores"])
	must.NotEq(t, "", node.Attributes["cpu.arch"])
	must.MapNotContainsKey(t, node.Attributes, "memory.totalbytes")
	must.MapNotContainsKey(t, node.Attributes, "os.name")
}
